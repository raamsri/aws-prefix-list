# AWS Prefix List Creator

## Overview

The AWS Prefix List Creator is a Go-based tool designed to simplify the process of creating AWS Managed Prefix Lists with large number of entries.

## Note

This is not operationally robust. For the sake of simplicity, the prefix list reference is "name", not a prefix list ID. Incidently, this may cause troubles with updates. Caution is advised.

## Methods

- **Resist the impulse to hate AWS for making this process very 
  tedious**: Patience seems to be a virtue
- **Chunking Entries**: The script divides the entries into manageable 
  chunks to avoid exceeding AWS limits(100 per request). 
- **State Management**: It checks the state of the AWS Prefix List to ensure that requests are not made while it is in a modifying state.
- **Retry Mechanism**: Implements multiple request cycles to handle large numbers of entries efficiently.

## Configuration

1. **AWS Credentials**: Configure your AWS credentials using the AWS CLI or by setting environment variables.
2. **Define CIDR Blocks**: Prepare a file containing the list of CIDR blocks (IP addresses) you want to include in the prefix list.

### Running the Script

1. **Build the Project**: Compile the Go code using the following command:
    ```sh
    go build -o aws_prefix_list_creator main.go
    ```

2. **Execute the Script**: Run the compiled binary with the required flags:
    ```sh
    ./aws_prefix_list_creator -action <create|update> -name <prefix_list_name> -file <path_to_ip_file>
    ```

    - `-action`: The action to perform, either `create` or `update`.
    - `-name`: The name of the prefix list.
    - `-file`: The path to the file containing the IP addresses.

## Detailed Description

### Main Function

The `main` function parses command-line flags to determine the action (`create` or `update`), the name of the prefix list, and the path to the file containing IP addresses. It then reads the IP addresses from the file and initializes the AWS SDK configuration.

### Reading IPs from File

The `readIPsFromFile` function reads IP addresses from the specified file, categorizing them into IPv4 and IPv6 addresses. It ensures that duplicate IP addresses are not included.

### Creating Prefix Lists

The `createPrefixList` function creates a new AWS Managed Prefix List. It handles large lists of IP addresses by splitting them into chunks and making multiple requests to AWS. It waits for the prefix list to be ready before making further modifications.

### Updating Prefix Lists

The `updatePrefixList` function updates an existing AWS Managed Prefix List. It determines which IP addresses need to be added or removed and updates the prefix list in chunks. It also waits for the prefix list to be ready before making further modifications.

### Helper Functions

- `isIPv4` and `isIPv6`: Determine whether a given IP address is IPv4 or IPv6.
- `getCurrentVersion`: Fetches the current version of a prefix list.
- `waitForPrefixListReady`: Waits for the prefix list to be ready for modifications.

This project simplifies the process of creating Prefix Lists on AWS, ensuring consistency and reducing the potential for human error.

### Sample Output

```sh
➜ head -n10 ../../ip.list
102.132.100.0/24
102.132.101.0/24
102.132.103.0/24
102.132.104.0/24
102.132.96.0/20
102.132.96.0/24
102.132.97.0/24
102.132.99.0/24
103.4.96.0/22
129.134.0.0/16

➜ wc -l ../../ip.list
     920 ../../ip.list
```

```sh
➜ go run main.go -action="create" -name="whatsapp-webhooks" -file="/home/ip.list"
2024/10/30 19:45:12 Action: create
2024/10/30 19:45:12 Prefix list name: whatsapp-webhooks
2024/10/30 19:45:12 File path: /home/ip.list
Created prefix list with ID: pl-089c9c21b707c58b6
2024/10/30 19:45:13 Prefix list state: create-in-progress
2024/10/30 19:45:18 Prefix list state: create-complete
Updated prefix list with ID: pl-089c9c21b707c58b6
2024/10/30 19:45:18 Prefix list state: modify-in-progress
2024/10/30 19:45:24 Prefix list state: modify-complete
Updated prefix list with ID: pl-089c9c21b707c58b6
2024/10/30 19:45:24 Prefix list state: modify-in-progress
2024/10/30 19:45:29 Prefix list state: modify-complete
Updated prefix list with ID: pl-089c9c21b707c58b6
2024/10/30 19:45:30 Prefix list state: modify-in-progress
2024/10/30 19:45:35 Prefix list state: modify-complete
Created prefix list with ID: pl-01e54ab870e7548c5
2024/10/30 19:45:35 Prefix list state: create-in-progress
2024/10/30 19:45:40 Prefix list state: create-complete
Updated prefix list with ID: pl-01e54ab870e7548c5
2024/10/30 19:45:41 Prefix list state: modify-in-progress
2024/10/30 19:45:46 Prefix list state: modify-complete
Updated prefix list with ID: pl-01e54ab870e7548c5
2024/10/30 19:45:47 Prefix list state: modify-in-progress
2024/10/30 19:45:52 Prefix list state: modify-complete
Updated prefix list with ID: pl-01e54ab870e7548c5
2024/10/30 19:45:52 Prefix list state: modify-in-progress
2024/10/30 19:45:57 Prefix list state: modify-complete
Updated prefix list with ID: pl-01e54ab870e7548c5
2024/10/30 19:45:58 Prefix list state: modify-in-progress
2024/10/30 19:46:03 Prefix list state: modify-complete
Updated prefix list with ID: pl-01e54ab870e7548c5
2024/10/30 19:46:04 Prefix list state: modify-in-progress
2024/10/30 19:46:09 Prefix list state: modify-complete
```

### TODO

- Create multiple prefix lists to limit each list with 'm' entries
- Create multiple security groups to fit the created Prefix Lists? A SG can't have more than 60 entries, so including a prefix list with large set of IPs is not possible. 
- Work with Prefix List ID rather than name

## License

This project is licensed under the MIT License.
