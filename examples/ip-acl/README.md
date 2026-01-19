# IP-based Access Control

This example demonstrates how to use network ACLs for IP-based access control with both simple IP checks and ACL-protected network listeners.

## Running the Example

```bash
go run github.com/dioad/net/examples/ip-acl
```

Or build and run:
```bash
cd examples/ip-acl
go build
./ip-acl
```

## What It Demonstrates

### Part 1: IP Authorization Checks
- Creating a network ACL with allowed and denied IP ranges
- Checking if specific IPs are authorized
- Using CIDR notation for network ranges
- Explicit IP blocking within allowed ranges

### Part 2: ACL-Protected Network Listeners
- Two TCP listeners protected by different ACL policies
- **Listener 1 (Port 9001)**: ALLOWS localhost connections (127.0.0.0/8)
- **Listener 2 (Port 9002)**: DENIES localhost connections (127.0.0.0/8)
- Real-time logging of allowed and denied connections
- Interactive TCP echo server for authorized connections

## Testing the Listeners

After running the example, test the ACL behavior with:

### Using netcat (nc):
```bash
# Test the ALLOW listener (should connect successfully)
nc localhost 9001

# Test the DENY listener (connection will be rejected)
nc localhost 9002
```

### Using telnet:
```bash
# Test the ALLOW listener
telnet localhost 9001

# Test the DENY listener
telnet localhost 9002
```

### Expected Behavior

**Port 9001 (ALLOW listener):**
- Connection succeeds
- You'll see a welcome message
- Can type messages that echo back
- Logs show "Connection ALLOWED"

**Port 9002 (DENY listener):**
- Connection is immediately closed by the ACL
- Logs show "Connection DENIED by ACL"
- No welcome message received

## Code

See [main.go](main.go) for the complete executable example.
