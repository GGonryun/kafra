# P0 SSH Agent - API Examples

This document provides curl examples for testing the P0 SSH Agent API endpoints. The agent receives JSON-RPC 2.0 requests over WebSocket connections and executes provisioning scripts based on the request data.

## Client Connection Format

The client ID format is: `{orgId}:{hostId}:ssh`

Example: `my-org:12345678-1234-5678-9abc-123456789def:ssh`

## Base URL Format

```
http://localhost:8081/client/{clientId}
```

Where `{clientId}` follows the format above.

## Available Provisioning Commands

The P0 SSH Agent supports the following provisioning commands:

- `provisionUser` - Create and manage user accounts
- `provisionAuthorizedKeys` - Manage SSH authorized keys
- `provisionSudo` - Manage sudo access permissions
- `provisionSession` - Terminate SSH sessions (revoke only)

## Request Format

All requests should include a `command` field in the data object along with the required parameters for that command.

### Common Parameters

All provisioning requests support these parameters:

- `userName` (string, required) - Target username (format: `^[a-z][-a-z0-9_]*$`)
- `action` (string, required) - Either "grant" or "revoke"
- `requestId` (string, required) - P0 access request identifier for tracking
- `publicKey` (string, optional) - SSH public key for authorized keys operations (required for grant actions, not required for revoke)
- `sudo` (boolean, optional) - Whether to grant sudo access

## Example Requests

### 1. Basic Connection Test

Test basic connectivity to the agent:

```bash
curl -v "http://localhost:8081/client/my-org:12345678-1234-5678-9abc-123456789def:ssh" \
  -H "Content-Type: application/json" \
  -d '{"hello":"world"}'
```

### 2. Provision User Account

Create a new user account:

```bash
curl -v "http://localhost:8081/client/my-org:12345678-1234-5678-9abc-123456789def:ssh" \
  -H "Content-Type: application/json" \
  -d '{
    "command": "provisionUser",
    "userName": "testuser",
    "action": "grant",
    "requestId": "req-12345"
  }'
```

### 3. Add SSH Public Key

Add SSH public key to user's authorized_keys:

```bash
curl -v "http://localhost:8081/client/my-org:12345678-1234-5678-9abc-123456789def:ssh" \
  -H "Content-Type: application/json" \
  -d '{
    "command": "provisionAuthorizedKeys",
    "userName": "testuser",
    "action": "grant",
    "requestId": "req-12345",
    "publicKey": "ssh-ed25519 AAAAC3Nz... user@example.com"
  }'
```

### 4. Grant Sudo Access

Grant passwordless sudo access to a user:

```bash
curl -v "http://localhost:8081/client/my-org:12345678-1234-5678-9abc-123456789def:ssh" \
  -H "Content-Type: application/json" \
  -d '{
    "command": "provisionSudo",
    "userName": "testuser",
    "action": "grant",
    "requestId": "req-12345",
    "sudo": true
  }'
```

### 5. Revoke SSH Key Access

Remove SSH key from user's authorized_keys using the original request ID (public key not required):

```bash
curl -v "http://localhost:8081/client/my-org:12345678-1234-5678-9abc-123456789def:ssh" \
  -H "Content-Type: application/json" \
  -d '{
    "command": "provisionAuthorizedKeys",
    "userName": "testuser",
    "action": "revoke",
    "requestId": "req-12345"
  }'
```

**Note:** Revoke operations use the `requestId` to identify which SSH key to remove, so the `publicKey` parameter is not required.

### 6. Revoke Sudo Access

Remove sudo access from a user:

```bash
curl -v "http://localhost:8081/client/my-org:12345678-1234-5678-9abc-123456789def:ssh" \
  -H "Content-Type: application/json" \
  -d '{
    "command": "provisionSudo",
    "userName": "testuser",
    "action": "revoke",
    "requestId": "req-12345"
  }'
```

### 7. Terminate SSH Sessions

Forcibly terminate all active SSH connections for a user (security/cleanup operation):

```bash
curl -v "http://localhost:8081/client/my-org:12345678-1234-5678-9abc-123456789def:ssh" \
  -H "Content-Type: application/json" \
  -d '{
    "command": "provisionSession",
    "userName": "example-user",
    "action": "revoke",
    "requestId": "req-emergency-001"
  }'
```

**Note:** The `provisionSession` command only supports the "revoke" action to terminate SSH connections. It finds and kills all SSH daemon processes for the specified user.

### 8. Emergency User Lockout

Complete user lockout by removing SSH access, sudo privileges, and terminating sessions:

```bash
# 1. Terminate active SSH sessions
curl -v "http://localhost:8081/client/my-org:12345678-1234-5678-9abc-123456789def:ssh" \
  -H "Content-Type: application/json" \
  -d '{
    "command": "provisionSession",
    "userName": "compromised-user",
    "action": "revoke",
    "requestId": "req-lockout-001"
  }'

# 2. Remove SSH key access
curl -v "http://localhost:8081/client/my-org:12345678-1234-5678-9abc-123456789def:ssh" \
  -H "Content-Type: application/json" \
  -d '{
    "command": "provisionAuthorizedKeys",
    "userName": "compromised-user",
    "action": "revoke",
    "requestId": "req-lockout-002"
  }'

# 3. Remove sudo access
curl -v "http://localhost:8081/client/my-org:12345678-1234-5678-9abc-123456789def:ssh" \
  -H "Content-Type: application/json" \
  -d '{
    "command": "provisionSudo",
    "userName": "compromised-user",
    "action": "revoke",
    "requestId": "req-lockout-003"
  }'
```

## Response Format

Successful responses return HTTP 200 with JSON data:

```json
{
  "success": true,
  "message": "User john-doe created successfully",
  "client_id": "my-org:12345678-1234-5678-9abc-123456789def:ssh",
  "command": "provisionUser",
  "timestamp": "2025-08-13T05:00:00Z",
  "status": "completed"
}
```

Failed responses return HTTP 500 with error details:

```json
{
  "success": false,
  "error": "user validation failed: invalid username format",
  "client_id": "my-org:12345678-1234-5678-9abc-123456789def:ssh",
  "command": "provisionUser",
  "timestamp": "2025-08-13T05:00:00Z",
  "status": "failed"
}
```

## Testing with Local Command Tool

You can also test the provisioning scripts locally using the built-in command tool:

```bash
# Test user provisioning (dry-run)
./dist/p0-ssh-agent command \
  --command provisionUser \
  --username testuser \
  --action grant \
  --request-id test-001 \
  --dry-run

# Test SSH key provisioning (grant - requires public key)
./dist/p0-ssh-agent command \
  --command provisionAuthorizedKeys \
  --username testuser \
  --action grant \
  --request-id test-002 \
  --public-key "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQC7W7Z8bF2Q9rK3pP1F7Xa8L5..." \
  --dry-run

# Test SSH key revocation (no public key needed)
./dist/p0-ssh-agent command \
  --command provisionAuthorizedKeys \
  --username testuser \
  --action revoke \
  --request-id test-002 \
  --dry-run

# Test sudo provisioning
./dist/p0-ssh-agent command \
  --command provisionSudo \
  --username testuser \
  --action grant \
  --request-id test-003 \
  --sudo \
  --dry-run

# Test SSH session termination
./dist/p0-ssh-agent command \
  --command provisionSession \
  --username testuser \
  --action revoke \
  --request-id test-004 \
  --dry-run
```

## Dry-Run Mode

The agent supports dry-run mode to test commands without making actual changes:

1. **Global dry-run**: Set `dryRun: true` in the config file
2. **Command-line dry-run**: Use `--dry-run` flag with the command tool
3. **Individual dry-run**: Currently not supported via API (would need to be added)

## Security Notes

- All provisioning operations require the agent to run with sudo privileges
- SSH keys are managed with proper file permissions (600)
- Sudo rules are stored in a separate file (`/etc/sudoers-p0`) with proper permissions (440)
- Request IDs provide audit trails for tracking access grants and revocations
- Username validation prevents privilege escalation through malicious usernames

## Troubleshooting

### Connection Issues

If curl requests fail to connect:

1. Verify the agent is running: `sudo systemctl status p0-ssh-agent`
2. Check agent logs: `sudo journalctl -u p0-ssh-agent -f`
3. Verify WebSocket endpoint is accessible
4. Check client ID format matches your configuration

### Script Execution Issues

If provisioning commands fail:

1. Check that the agent has sudo privileges
2. Verify required system commands exist (`useradd`, `mkdir`, `chmod`, etc.)
3. Check file permissions on key directories
4. Review agent logs for detailed error messages

### Request Format Issues

If requests are rejected:

1. Verify JSON syntax is valid
2. Check required fields are present (`userName`, `action`, `requestId`)
3. Ensure username follows the required pattern: `^[a-z][-a-z0-9_]*$`
4. Verify `action` is either "grant" or "revoke"

## Advanced Examples

### Testing with jq for Pretty Output

```bash
curl -s "http://localhost:8081/client/my-org:12345678-1234-5678-9abc-123456789def:ssh" \
  -H "Content-Type: application/json" \
  -d '{
    "command": "provisionUser",
    "userName": "test-user",
    "action": "grant",
    "requestId": "req-test-001"
  }' | jq '.'
```

### Batch Operations with Shell Script

```bash
#!/bin/bash

CLIENT_ID="my-org:12345678-1234-5678-9abc-123456789def:ssh"
BASE_URL="http://localhost:8081/client/$CLIENT_ID"
USERNAME="deployment-user"
REQUEST_ID="req-deploy-$(date +%s)"
PUBLIC_KEY="ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQC7W7Z8bF2Q9rK3pP1F7Xa8L5... deploy@example.com"

echo "Creating user..."
curl -s "$BASE_URL" \
  -H "Content-Type: application/json" \
  -d "{
    \"command\": \"provisionUser\",
    \"userName\": \"$USERNAME\",
    \"action\": \"grant\",
    \"requestId\": \"$REQUEST_ID\"
  }" | jq '.'

echo "Adding SSH key..."
curl -s "$BASE_URL" \
  -H "Content-Type: application/json" \
  -d "{
    \"command\": \"provisionAuthorizedKeys\",
    \"userName\": \"$USERNAME\",
    \"action\": \"grant\",
    \"requestId\": \"$REQUEST_ID\",
    \"publicKey\": \"$PUBLIC_KEY\"
  }" | jq '.'

echo "User provisioned successfully!"
```
