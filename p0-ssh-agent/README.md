# P0 SSH Agent

A comprehensive SSH access management tool that connects to P0 backend infrastructure via WebSocket, executes provisioning scripts, and provides secure authentication using JWT tokens with ECDSA P-384 keys.

## Features

- **JWT Authentication**: ES384 algorithm with ECDSA P-384 private keys
- **WebSocket Communication**: Secure WebSocket connections with JSON-RPC 2.0 protocol
- **SSH Provisioning**: Automated user, SSH key, and sudo access management
- **Dry-Run Mode**: Safe testing without making actual system changes
- **Systemd Integration**: Automated service installation and management
- **Command Testing**: Direct script execution for validation
- **Automatic Reconnection**: Exponential backoff retry mechanism for connection failures
- **Enhanced Debugging**: Detailed HTTP status code logging for WebSocket connection issues
- **Secure Key Management**: Separate key generation with protection against accidental recreation

## Quick Start

### 1. Build the Tool

```bash
# Build the single binary with all subcommands
make build
```

The binary will be created as `bin/p0-ssh-agent`.

### 2. Generate JWT Keys

**⚠️ Important**: Only run this once per client. Regenerating keys will break existing registrations.

```bash
# Generate keys in current directory
./bin/p0-ssh-agent keygen

# Or specify a custom path
./bin/p0-ssh-agent keygen --key-path ~/.p0/keys
```

The keygen command will:
- Generate ECDSA P-384 private/public keypair
- Save `jwk.private.json` and `jwk.public.json`
- Display the public key for backend registration
- Protect against accidental overwriting (use `--force` to override)

### 3. Create Configuration

Create a configuration file `p0-ssh-agent.yaml`:

```yaml
version: "1.0"
orgId: "my-company"
hostId: "dev-machine-01"
tunnelHost: "wss://p0.example.com/websocket"
keyPath: "/etc/p0-ssh-agent/keys"
logPath: "/var/log/p0-ssh-agent"
labels:
  - "type=development"
  - "owner=local-admin"
environment: "development"
tunnelTimeoutMs: 30000
```

### 4. Register with Backend

Generate a registration request:

```bash
./bin/p0-ssh-agent register --config p0-ssh-agent.yaml
```

### 5. Start the Agent

```bash
# Using configuration file
./bin/p0-ssh-agent start --config p0-ssh-agent.yaml

# Or with command line flags
./bin/p0-ssh-agent start \
  --org-id my-company \
  --host-id dev-machine-01 \
  --tunnel-host wss://p0.example.com/websocket \
  --key-path ~/.p0/keys
```

## Available Commands

### Global Flags
| Flag | Description | Default |
|------|-------------|---------|
| `-c, --config` | Path to configuration file | - |
| `-v, --verbose` | Enable verbose logging | `false` |

### `start` - Start the SSH Agent
Start the WebSocket proxy agent that connects to P0 backend.

| Flag | Description | Default |
|------|-------------|---------|
| `--org-id` | Organization identifier (required) | - |
| `--host-id` | Host identifier (required) | - |
| `--tunnel-host` | WebSocket URL (e.g., ws://localhost:8079) | - |
| `--key-path` | Path to store JWT key files | - |
| `--log-path` | Path to store log files (for daemon mode) | - |
| `--labels` | Machine labels for registration | - |
| `--environment` | Environment ID for registration | - |
| `--tunnel-timeout` | Tunnel timeout in milliseconds | - |
| `--dry-run` | Log commands but don't execute them | `false` |

### `keygen` - Generate JWT Keys
Generate ECDSA P-384 keypair for JWT authentication.

| Flag | Description | Default |
|------|-------------|---------|
| `--key-path` | Directory to store key files | `.` |
| `--force` | Overwrite existing keys | `false` |

### `register` - Generate Registration Request
Generate machine registration request for P0 backend.

| Flag | Description | Default |
|------|-------------|---------|
| `--output` | Output format (json or yaml) | `json` |

### `install` - Install as Systemd Service
Install P0 SSH Agent as a systemd service for automatic startup.

| Flag | Description | Default |
|------|-------------|---------|
| `--service-name` | Name for the systemd service | `p0-ssh-agent` |
| `--user` | User to run the service as | `p0-agent` |
| `--executable` | Path to executable (auto-detected) | - |
| `--working-dir` | Working directory for service | `/etc/p0-ssh-agent` |
| `--config-file` | Configuration file path | `/etc/p0-ssh-agent/p0-ssh-agent.yaml` |

### `command` - Execute Provisioning Scripts
Execute provisioning scripts directly for testing and validation.

| Flag | Description | Default |
|------|-------------|---------|
| `--command` | Command to execute (required) | - |
| `--username` | Username for the operation (required) | - |
| `--action` | Action to perform (grant or revoke) | `grant` |
| `--request-id` | Request ID for tracking | auto-generated |
| `--public-key` | SSH public key for authorized keys | - |
| `--sudo` | Grant sudo access | `false` |
| `--dry-run` | Log commands but don't execute them | `false` |

#### Available Commands for `command`:
- `provisionUser` - Create/remove user accounts
- `provisionAuthorizedKeys` - Manage SSH authorized keys
- `provisionSudo` - Grant/revoke sudo access

## Usage Examples

### Basic Setup and Usage

```bash
# 1. Generate JWT keys
./bin/p0-ssh-agent keygen --key-path ~/.p0/keys

# 2. Create configuration file
cat > p0-ssh-agent.yaml << EOF
version: "1.0"
orgId: "my-company"
hostId: "$(hostname)"
tunnelHost: "wss://p0.example.com/websocket"
keyPath: "~/.p0/keys"
environment: "production"
tunnelTimeoutMs: 30000
EOF

# 3. Generate registration request
./bin/p0-ssh-agent register --config p0-ssh-agent.yaml

# 4. Start the agent
./bin/p0-ssh-agent start --config p0-ssh-agent.yaml --verbose
```

### Command Line Usage

```bash
# Start with individual flags
./bin/p0-ssh-agent start \
  --org-id my-company \
  --host-id dev-machine-01 \
  --tunnel-host wss://p0.example.com/websocket \
  --key-path ~/.p0/keys \
  --verbose

# Start with dry-run mode (safe testing)
./bin/p0-ssh-agent start --config p0-ssh-agent.yaml --dry-run

# Force regenerate keys (dangerous!)
./bin/p0-ssh-agent keygen --key-path ~/.p0/keys --force
```

### Testing and Validation

```bash
# Test user provisioning
./bin/p0-ssh-agent command \
  --command provisionUser \
  --username testuser \
  --dry-run

# Test SSH key provisioning
./bin/p0-ssh-agent command \
  --command provisionAuthorizedKeys \
  --username testuser \
  --public-key "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC7..." \
  --dry-run

# Test sudo access
./bin/p0-ssh-agent command \
  --command provisionSudo \
  --username testuser \
  --sudo \
  --action grant \
  --dry-run

# Test revoke operations
./bin/p0-ssh-agent command \
  --command provisionUser \
  --username testuser \
  --action revoke \
  --dry-run
```

### Production Deployment

```bash
# Install as systemd service
sudo ./bin/p0-ssh-agent install \
  --service-name p0-ssh-agent \
  --user p0-agent \
  --working-dir /etc/p0-ssh-agent \
  --config-file /etc/p0-ssh-agent/p0-ssh-agent.yaml

# The install command provides complete setup instructions
```

### Help and Documentation

```bash
# Show help for main command
./bin/p0-ssh-agent --help

# Show help for specific subcommands
./bin/p0-ssh-agent start --help
./bin/p0-ssh-agent command --help
./bin/p0-ssh-agent install --help
```

## Architecture

### Authentication Flow
1. Agent loads ECDSA P-384 private key from `jwk.private.json`
2. Creates JWT token with ES384 algorithm and client ID
3. Establishes WebSocket connection with `Authorization: Bearer <token>` header
4. Sends `setClientId` RPC call to register with backend

### Request Handling
- Receives `call` method requests via JSON-RPC 2.0
- Extracts commands from request.Data["command"]
- Executes appropriate provisioning scripts (user, SSH keys, sudo)
- Supports dry-run mode for safe testing
- Logs request details and execution results
- Filters sensitive headers from logs (e.g., authorization)

### Connection Management
- Automatic reconnection with exponential backoff (1s to 30s)
- Graceful shutdown on SIGINT/SIGTERM
- Connection status monitoring and detailed error reporting

## Command Reference

The p0-ssh-agent binary includes multiple subcommands:

### Available Commands
- `start` - Start the WebSocket proxy agent
- `keygen` - Generate JWT keypair for authentication
- `register` - Generate machine registration request
- `install` - Install as systemd service
- `command` - Execute provisioning scripts directly
- `help` - Show help information

### Build Options

The included Makefile provides several build targets:

```bash
make build     # Build p0-ssh-agent binary with subcommands (default)
make deps      # Install Go dependencies
make test      # Run tests
make clean     # Remove build artifacts
make install   # Install to /usr/local/bin (requires sudo)
make dev       # Development build without optimization
make help      # Show all available targets
```

## Troubleshooting

### Common Issues

**WebSocket connection fails with "bad handshake":**
- Check authentication with `--verbose` flag
- Verify client ID is registered in backend
- Ensure JWT keys exist and are readable
- Test endpoint with: `npx wscat -c ws://localhost:8079/socket`

**"Failed to load JWT key" error:**
```bash
# Generate keys first
./bin/p0-ssh-agent keygen --key-path ~/.p0/keys

# Then run agent
./bin/p0-ssh-agent start --org-id my-org --host-id my-host --key-path ~/.p0/keys
```

**Permission denied errors:**
```bash
# Create secure key directory
mkdir -p ~/.p0/keys
chmod 700 ~/.p0/keys

# Generate keys there
./bin/p0-ssh-agent keygen --key-path ~/.p0/keys
```

### Debug Logging

Enable verbose logging to see detailed connection information:

```bash
./bin/p0-ssh-agent start --config p0-ssh-agent.yaml --verbose
```

This shows:
- WebSocket connection attempts with URLs
- HTTP status codes for failed handshakes  
- JWT token creation (redacted for security)
- RPC message handling details

## Deployment

### Production Installation

```bash
# Build and install system-wide
make install

# Run from anywhere
p0-ssh-agent start --config /etc/p0-ssh-agent/p0-ssh-agent.yaml
```

### Systemd Service (Automated)

Use the built-in install command for automated systemd setup:

```bash
# Generate service files and installation instructions
sudo p0-ssh-agent install \
  --service-name p0-ssh-agent \
  --user p0-agent \
  --working-dir /etc/p0-ssh-agent \
  --config-file /etc/p0-ssh-agent/p0-ssh-agent.yaml

# Follow the printed instructions to complete installation
```

The install command automatically:
- Generates systemd service file with security hardening
- Creates working directories with proper permissions
- Provides step-by-step installation instructions
- Includes service management commands

## Security Notes

- **Private Keys**: Stored locally and never transmitted over the network
- **JWT Tokens**: Short-lived and created per connection attempt
- **Log Filtering**: Authorization headers and sensitive data filtered from logs
- **File Permissions**: Key files should have restrictive permissions (600/700)
- **Transport Security**: Use `wss://` (secure WebSocket) for production deployments
- **Dry-Run Mode**: Test commands safely without making system changes
- **Systemd Hardening**: Install command includes security settings like `NoNewPrivileges`, `ProtectSystem=strict`
- **Privilege Separation**: Service runs as dedicated non-root user
- **Provisioning Safety**: All script operations are logged and can be tested in dry-run mode

## Configuration File Format

The configuration file uses YAML format with the following structure:

```yaml
# Required fields
version: "1.0"
orgId: "organization-name"           # Your organization identifier
hostId: "machine-hostname"           # Unique host identifier
tunnelHost: "wss://p0.example.com"   # WebSocket URL (ws:// or wss://)

# Optional fields
keyPath: "/path/to/keys"             # JWT key storage directory
logPath: "/path/to/logs"             # Log file directory (empty = stdout)
environment: "production"            # Environment identifier
tunnelTimeoutMs: 30000               # Connection timeout in milliseconds
dryRun: false                        # Enable dry-run mode globally

# Machine labels (optional)
labels:
  - "type=production"
  - "region=us-west-2"
  - "team=infrastructure"
```