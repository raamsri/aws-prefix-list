package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func main() {
	action := flag.String("action", "create", "Action to perform: create or update")
	prefixListName := flag.String("name", "", "Name of the prefix list")
	filePath := flag.String("file", "", "Path to the file containing IPs")
	flag.Parse()
	log.Printf("Action: %s\n", *action)
	log.Printf("Prefix list name: %s\n", *prefixListName)
	log.Printf("File path: %s\n", *filePath)

	if *prefixListName == "" || *filePath == "" {
		log.Fatal("Prefix list name and file path are required")
	}

	ipv4s, ipv6s, err := readIPsFromFile(*filePath)
	if err != nil {
		log.Fatalf("Failed to read IPs from file: %v", err)
	}

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}

	svc := ec2.NewFromConfig(cfg)

	switch *action {
	case "create":
		createPrefixList(svc, *prefixListName+"-ipv4", "IPv4", ipv4s)
		createPrefixList(svc, *prefixListName+"-ipv6", "IPv6", ipv6s)
	case "update":
		updatePrefixList(svc, *prefixListName+"-ipv4", ipv4s)
		updatePrefixList(svc, *prefixListName+"-ipv6", ipv6s)
	default:
		log.Fatalf("Unknown action: %s", *action)
	}
}

func readIPsFromFile(filePath string) ([]string, []string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	ipv4Set := make(map[string]struct{})
	ipv6Set := make(map[string]struct{})
	var ipv4s, ipv6s []string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		ip := strings.TrimSpace(scanner.Text())
		if ip != "" {
			if isIPv4(ip) {
				if _, exists := ipv4Set[ip]; !exists {
					ipv4Set[ip] = struct{}{}
					ipv4s = append(ipv4s, ip)
				}
			} else if isIPv6(ip) {
				if _, exists := ipv6Set[ip]; !exists {
					ipv6Set[ip] = struct{}{}
					ipv6s = append(ipv6s, ip)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}

	return ipv4s, ipv6s, nil
}

func isIPv4(ip string) bool {
	_, _, err := net.ParseCIDR(ip)
	return err == nil && strings.Contains(ip, ".")
}

func isIPv6(ip string) bool {
	_, _, err := net.ParseCIDR(ip)
	return err == nil && strings.Contains(ip, ":")
}

func createPrefixList(svc *ec2.Client, name, addressFamily string, ips []string) {
	const maxEntriesPerRequest = 100
	totalEntries := len(ips)
	numRequests := (totalEntries + maxEntriesPerRequest - 1) / maxEntriesPerRequest

	var prefixListID string
	var currentVersion int64 = 1

	for i := 0; i < numRequests; i++ {
		start := i * maxEntriesPerRequest
		end := start + maxEntriesPerRequest
		if end > totalEntries {
			end = totalEntries
		}

		entries := make([]types.AddPrefixListEntry, end-start)
		for j, ip := range ips[start:end] {
			entries[j] = types.AddPrefixListEntry{
				Cidr: aws.String(ip),
			}
		}

		if i == 0 {
			input := &ec2.CreateManagedPrefixListInput{
				PrefixListName: aws.String(name),
				AddressFamily:  aws.String(addressFamily),
				MaxEntries:     aws.Int32(int32(totalEntries)), // Set MaxEntries to total number of entries
				Entries:        entries,
			}

			result, err := svc.CreateManagedPrefixList(context.TODO(), input)
			if err != nil {
				log.Fatalf("Failed to create prefix list: %v", err)
			}
			prefixListID = *result.PrefixList.PrefixListId
			currentVersion = *result.PrefixList.Version
			fmt.Printf("Created prefix list with ID: %s\n", prefixListID)
		} else {
			// Fetch the latest version before each modification
			currentVersion = getCurrentVersion(svc, prefixListID)

			updateInput := &ec2.ModifyManagedPrefixListInput{
				PrefixListId:   aws.String(prefixListID),
				CurrentVersion: aws.Int64(currentVersion),
				AddEntries:     entries,
			}
			result, err := svc.ModifyManagedPrefixList(context.TODO(), updateInput)
			if err != nil {
				log.Fatalf("Failed to update prefix list: %v", err)
			}
			currentVersion = *result.PrefixList.Version
			fmt.Printf("Updated prefix list with ID: %s\n", prefixListID)
		}

		// Wait for the prefix list to be ready for the next modification
		waitForPrefixListReady(svc, prefixListID)
	}
}

func updatePrefixList(svc *ec2.Client, name string, ips []string) {
	const maxEntriesPerRequest = 100

	// Find the prefix list by name
	describeInput := &ec2.DescribeManagedPrefixListsInput{}
	describeResult, err := svc.DescribeManagedPrefixLists(context.TODO(), describeInput)
	if err != nil {
		log.Fatalf("Failed to describe prefix lists: %v", err)
	}

	var prefixListID string
	for _, pl := range describeResult.PrefixLists {
		if *pl.PrefixListName == name {
			prefixListID = *pl.PrefixListId
			break
		}
	}

	if prefixListID == "" {
		log.Fatalf("Prefix list with name %s not found", name)
	}

	// Get current version of the prefix list
	currentVersion := getCurrentVersion(svc, prefixListID)

	// Determine entries to add and remove
	currentEntries := make(map[string]bool)
	entriesInput := &ec2.GetManagedPrefixListEntriesInput{
		PrefixListId: aws.String(prefixListID),
	}
	entriesResult, err := svc.GetManagedPrefixListEntries(context.TODO(), entriesInput)
	if err != nil {
		log.Fatalf("Failed to get prefix list entries: %v", err)
	}
	for _, entry := range entriesResult.Entries {
		currentEntries[*entry.Cidr] = true
	}

	var addEntries []types.AddPrefixListEntry
	var removeEntries []types.RemovePrefixListEntry

	for _, ip := range ips {
		if !currentEntries[ip] {
			addEntries = append(addEntries, types.AddPrefixListEntry{
				Cidr: aws.String(ip),
			})
		}
		delete(currentEntries, ip)
	}

	for ip := range currentEntries {
		removeEntries = append(removeEntries, types.RemovePrefixListEntry{
			Cidr: aws.String(ip),
		})
	}

	// Update the prefix list in chunks
	for i := 0; i < len(addEntries) || i < len(removeEntries); i += maxEntriesPerRequest {
		endAdd := i + maxEntriesPerRequest
		if endAdd > len(addEntries) {
			endAdd = len(addEntries)
		}
		endRemove := i + maxEntriesPerRequest
		if endRemove > len(removeEntries) {
			endRemove = len(removeEntries)
		}

		// Fetch the latest version before each modification
		currentVersion = getCurrentVersion(svc, prefixListID)

		updateInput := &ec2.ModifyManagedPrefixListInput{
			PrefixListId:   aws.String(prefixListID),
			CurrentVersion: aws.Int64(currentVersion),
			AddEntries:     addEntries[i:endAdd],
			RemoveEntries:  removeEntries[i:endRemove],
		}

		result, err := svc.ModifyManagedPrefixList(context.TODO(), updateInput)
		if err != nil {
			log.Fatalf("Failed to update prefix list: %v", err)
		}
		currentVersion = *result.PrefixList.Version
		fmt.Printf("Updated prefix list with ID: %s\n", prefixListID)

		// Wait for the prefix list to be ready for the next modification
		waitForPrefixListReady(svc, prefixListID)
	}
}

func getCurrentVersion(svc *ec2.Client, prefixListID string) int64 {
	describeInput := &ec2.DescribeManagedPrefixListsInput{
		PrefixListIds: []string{prefixListID},
	}
	describeResult, err := svc.DescribeManagedPrefixLists(context.TODO(), describeInput)
	if err != nil {
		log.Fatalf("Failed to describe prefix list: %v", err)
	}
	return *describeResult.PrefixLists[0].Version
}

func waitForPrefixListReady(svc *ec2.Client, prefixListID string) {
	for {
		describeInput := &ec2.DescribeManagedPrefixListsInput{
			PrefixListIds: []string{prefixListID},
		}
		describeResult, err := svc.DescribeManagedPrefixLists(context.TODO(), describeInput)
		if err != nil {
			log.Fatalf("Failed to describe prefix list: %v", err)
		}
		log.Printf("Prefix list state: %s\n", describeResult.PrefixLists[0].State)

		currentState := string(describeResult.PrefixLists[0].State)

		if len(describeResult.PrefixLists) > 0 && !strings.Contains(currentState, "-in-progress") {
			break
		}

		time.Sleep(5 * time.Second) // Wait for 5 seconds before checking again
	}
}
