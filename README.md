# P0 SSH Agent

A comprehensive SSH access management tool for on-premises nodes that connects to P0 backend infrastructure via WebSocket, executes provisioning scripts, and provides secure authentication using JWT tokens with ECDSA P-384 keys.

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

## Quick Start (On-Premises Setup)

### Recommended Installation Process

The complete setup process for P0 SSH Agent on an on-premises node:

```bash
# 1. Download or build the binary
wget https://releases.p0.com/p0-ssh-agent/latest/p0-ssh-agent-linux-amd64
chmod +x p0-ssh-agent-linux-amd64
mv p0-ssh-agent-linux-amd64 p0-ssh-agent

# 2. Install system components (copies binary, creates service, generates keys)
sudo ./p0-ssh-agent install

# 3. Configure your organization settings
sudo vi /etc/p0-ssh-agent/config.yaml
# Update: orgId, hostId, tunnelHost

# 4. Register with P0 backend (generates registration code)
sudo p0-ssh-agent register --config /etc/p0-ssh-agent/config.yaml

# 5. After registration approval, start the service
sudo systemctl enable p0-ssh-agent
sudo systemctl start p0-ssh-agent
```

### Method 2: Manual Build and Setup

If you need to build from source:

```bash
# Build the single binary with all subcommands
make build
```

The binary will be created as `dist/p0-ssh-agent`.

### Manual Configuration Steps

#### 1. Generate JWT Keys

**⚠️ Important**: Only run this once per client. Regenerating keys will break existing registrations.

```bash
# Generate keys in current directory
./dist/p0-ssh-agent keygen

# Or specify a custom path
./dist/p0-ssh-agent keygen --key-path ~/.p0/keys
```

The keygen command will:

- Generate ECDSA P-384 private/public keypair
- Save `jwk.private.json` and `jwk.public.json`
- Display the public key for backend registration
- Protect against accidental overwriting (use `--force` to override)

#### 2. Create Configuration

Create a configuration file `config.yaml`:

```yaml
version: "1.0"
orgId: "my-company" # Replace with your organization ID
hostId: "hostname-goes-here" # Replace with unique host identifier
hostname: "custom-hostname" # Optional: override system hostname
tunnelHost: "wss://api.p0.app" # P0 backend URL
keyPath: "/etc/p0-ssh-agent/keys" # JWT key storage directory
labels:
  - "environment=production" # Machine labels for identification
  - "team=infrastructure"
  - "region=us-west-2"
environment: "production" # Environment identifier
```

#### 3. Register with Backend

Generate a registration request:

```bash
./dist/p0-ssh-agent register --config config.yaml
```

#### 4. Start the Agent

```bash
# Using configuration file
./dist/p0-ssh-agent start --config config.yaml

# Or with command line flags
./dist/p0-ssh-agent start \
  --org-id my-company \
  --host-id hostname-goes-here \
  --tunnel-host wss://p0.example.com/websocket \
  --key-path ~/.p0/keys
```

## On-Premises Node Configuration Requirements

### Required Configuration Fields

For on-premises nodes, these fields are **mandatory**:

- `orgId`: Your P0 organization identifier
- `hostId`: Unique identifier for this machine/node
- `tunnelHost`: WebSocket URL to your P0 backend (must be accessible from the node)

### Network Requirements

- **Outbound HTTPS/WSS**: The node must be able to reach your P0 backend on port 443
- **No Inbound Connections**: The agent initiates all connections outbound
- **WebSocket Support**: Ensure firewalls/proxies support WebSocket upgrades

### Security Considerations

- **JWT Keys**: Stored locally on the node, never transmitted
- **Secure Transport**: Always use `wss://` (secure WebSocket) for production
- **File Permissions**: Key files automatically set to 600/700 permissions
- **Service User**: Runs as dedicated `p0-agent` user (created during install)

## Available Commands

All commands support these global flags:
| Flag | Description | Default |
|------|-------------|---------|
| `-c, --config` | Path to configuration file | - |
| `-v, --verbose` | Enable verbose logging | `false` |


### `start` - Start the SSH Agent

Start the WebSocket proxy agent that connects to P0 backend.

| Flag               | Description                               | Default |
| ------------------ | ----------------------------------------- | ------- |
| `--org-id`         | Organization identifier (required)        | -       |
| `--host-id`        | Host identifier (required)                | -       |
| `--tunnel-host`    | WebSocket URL (e.g., ws://localhost:8079) | -       |
| `--key-path`       | Path to store JWT key files               | -       |
| `--labels`         | Machine labels for registration           | -       |
| `--environment`    | Environment ID for registration           | -       |
| `--tunnel-timeout` | Tunnel timeout in milliseconds            | -       |
| `--dry-run`        | Log commands but don't execute them       | `false` |

### `keygen` - Generate JWT Keys

Generate ECDSA P-384 keypair for JWT authentication.

| Flag         | Description                  | Default |
| ------------ | ---------------------------- | ------- |
| `--key-path` | Directory to store key files | `.`     |
| `--force`    | Overwrite existing keys      | `false` |

### `register` - Generate Registration Request

Generate machine registration request for P0 backend.

| Flag       | Description                  | Default |
| ---------- | ---------------------------- | ------- |
| `--output` | Output format (json or yaml) | `json`  |

### `install` - Install as Systemd Service

Complete P0 SSH Agent installation as a systemd service with full setup.

**Usage:**

```bash
sudo p0-ssh-agent install
```

| Flag             | Description                  | Default        |
| ---------------- | ---------------------------- | -------------- |
| `--service-name` | Name for the systemd service | `p0-ssh-agent` |
| `--user`         | User to run the service as   | `p0-agent`     |

**What it does (comprehensive setup):**

- Copies binary to system location (`/usr/local/bin/p0-ssh-agent`)
- Creates configuration directory and default config file
- Creates service user (`p0-agent`)
- Creates directories with proper permissions
- Generates JWT keys automatically
- Creates systemd service file (not started)
- **NixOS**: Creates NixOS module at `/etc/nixos/p0-ssh-agent.nix`
- Provides instructions for configuration and registration

**After installation, you must:**
1. Edit the configuration file with your settings
2. Run the register command to get your registration code
3. Start the service after approval

**Perfect for production on-premises deployments.**

#### NixOS Installation

For NixOS systems, the install command provides special handling:

```bash
sudo ./p0-ssh-agent install
```

**NixOS-specific setup:**
- Creates NixOS module in system modules path
- Module contains systemd service definition with correct paths
- Installs module to `/etc/nixos/modules/jit/` for easy import

**After installation, add to your `/etc/nixos/configuration.nix`:**

```nix
{
  imports = [
    # ... your existing imports ...
    ./modules/jit/p0-ssh-agent.nix
  ];

  services.p0-ssh-agent.enable = true;
}
```

Then rebuild your system:
```bash
sudo nixos-rebuild switch
```

**Configuration**: Edit `/etc/p0-ssh-agent/config.yaml` (same as other platforms)

### `uninstall` - Completely Remove Installation

Remove all P0 SSH Agent components from the system.

**Usage:**

```bash
sudo p0-ssh-agent uninstall
```

| Flag             | Description                    | Default        |
| ---------------- | ------------------------------ | -------------- |
| `--service-name` | Name of systemd service        | `p0-ssh-agent` |
| `--user`         | Service user to remove         | `p0-agent`     |
| `--force`        | Skip confirmation prompts     | `false`        |

**What it removes:**

- Stops and disables systemd service
- Removes service files and configuration
- Removes service user and home directory
- Removes configuration directory (`/etc/p0-ssh-agent/`)
- Removes log directory (`/var/log/p0-ssh-agent/`)
- Removes system binary (`/usr/local/bin/p0-ssh-agent`)
- Cleans up all installation artifacts

**⚠️ WARNING:** This permanently deletes all configuration, keys, and logs.

### `status` - Check Installation Status

Comprehensive health check of your P0 SSH Agent installation.

**Usage:**

```bash
p0-ssh-agent status
sudo p0-ssh-agent status  # For full system check
```

**Validates:**

- Configuration file validity
- Service user existence and permissions
- JWT key presence and validity
- Log file accessibility
- Systemd service status
- Directory permissions and ownership

### `command` - Execute Provisioning Scripts

Execute provisioning scripts directly for testing and validation.

| Flag           | Description                           | Default        |
| -------------- | ------------------------------------- | -------------- |
| `--command`    | Command to execute (required)         | -              |
| `--username`   | Username for the operation (required) | -              |
| `--action`     | Action to perform (grant or revoke)   | `grant`        |
| `--request-id` | Request ID for tracking               | auto-generated |
| `--public-key` | SSH public key for authorized keys    | -              |
| `--sudo`       | Grant sudo access                     | `false`        |
| `--dry-run`    | Log commands but don't execute them   | `false`        |

#### Available Commands for `command`:

- `provisionUser` - Create/remove user accounts
- `provisionAuthorizedKeys` - Manage SSH authorized keys
- `provisionSudo` - Grant/revoke sudo access

## Usage Examples

### On-Premises Node Setup (Recommended)

**Complete automated setup:**

```bash
# 1. Install system components (creates everything needed)
sudo ./p0-ssh-agent install

# 2. Edit configuration with your details
sudo vi /etc/p0-ssh-agent/config.yaml
# Update: orgId, hostId, tunnelHost

# 3. Register with P0 backend
sudo p0-ssh-agent register --config /etc/p0-ssh-agent/config.yaml

# 4. After approval, start the service
sudo systemctl enable p0-ssh-agent
sudo systemctl start p0-ssh-agent

# 5. Check status
sudo p0-ssh-agent status
```

**Manual setup (if needed):**

```bash
# 1. Generate JWT keys
./dist/p0-ssh-agent keygen --key-path ~/.p0/keys

# 2. Create configuration file
cat > config.yaml << EOF
version: "1.0"
orgId: "my-company"
hostId: "$(hostname)"
tunnelHost: "wss://p0.example.com/websocket"
keyPath: "~/.p0/keys"
environment: "production"
tunnelTimeoutMs: 30000
EOF

# 3. Generate registration request
./dist/p0-ssh-agent register --config config.yaml

# 4. Start the agent
./dist/p0-ssh-agent start --config config.yaml --verbose
```

### Command Line Usage

```bash
# Start with individual flags
./dist/p0-ssh-agent start \
  --org-id my-company \
  --host-id hostname-goes-here \
  --tunnel-host wss://p0.example.com/websocket \
  --key-path ~/.p0/keys \
  --verbose

# Start with dry-run mode (safe testing)
./dist/p0-ssh-agent start --config config.yaml --dry-run

# Force regenerate keys (dangerous!)
./dist/p0-ssh-agent keygen --key-path ~/.p0/keys --force

# Check node status and health
p0-ssh-agent status --verbose
```

### Testing and Validation

```bash
# Test user provisioning
./dist/p0-ssh-agent command \
  --command provisionUser \
  --username testuser \
  --dry-run

# Test SSH key provisioning
./dist/p0-ssh-agent command \
  --command provisionAuthorizedKeys \
  --username testuser \
  --public-key "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC7..." \
  --dry-run

# Test sudo access
./dist/p0-ssh-agent command \
  --command provisionSudo \
  --username testuser \
  --sudo \
  --action grant \
  --dry-run

# Test revoke operations
./dist/p0-ssh-agent command \
  --command provisionUser \
  --username testuser \
  --action revoke \
  --dry-run
```

### Production On-Premises Deployment

**Recommended approach (install → configure → register → start):**

```bash
# 1. Install system components (copies binary, creates config template)
sudo ./p0-ssh-agent install

# 2. Edit configuration with your organization settings
sudo vi /etc/p0-ssh-agent/config.yaml

# 3. Register with P0 backend (provides registration code)
sudo p0-ssh-agent register --config /etc/p0-ssh-agent/config.yaml

# 4. After approval, start the service
sudo systemctl enable p0-ssh-agent
sudo systemctl start p0-ssh-agent
```

**Manual approach:**

```bash
# Install as systemd service manually
sudo ./dist/p0-ssh-agent install \
  --service-name p0-ssh-agent \
  --user p0-agent

# Check installation health
sudo p0-ssh-agent status

# Completely remove P0 SSH Agent
sudo p0-ssh-agent uninstall
```

### Help and Documentation

```bash
# Show help for main command
./dist/p0-ssh-agent --help

# Show help for specific subcommands
./dist/p0-ssh-agent start --help
./dist/p0-ssh-agent install --help
./dist/p0-ssh-agent status --help
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
- `install` - Complete systemd service installation
- `uninstall` - Completely remove P0 SSH Agent installation
- `status` - Check installation health and status
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
./dist/p0-ssh-agent keygen --key-path ~/.p0/keys

# Then run agent
./dist/p0-ssh-agent start --org-id my-org --host-id my-host --key-path ~/.p0/keys
```

**Permission denied errors:**

```bash
# Create secure key directory
mkdir -p ~/.p0/keys
chmod 700 ~/.p0/keys

# Generate keys there
./dist/p0-ssh-agent keygen --key-path ~/.p0/keys
```

### Debug Logging

Enable verbose logging to see detailed connection information:

```bash
./dist/p0-ssh-agent start --config config.yaml --verbose
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
p0-ssh-agent start --config /etc/p0-ssh-agent/config.yaml
```

### Systemd Service (Automated)

Use the built-in install command for automated systemd setup:

```bash
# After bootstrap, complete installation automatically
sudo p0-ssh-agent install

# Or with custom options
sudo p0-ssh-agent install \
  --service-name p0-ssh-agent \
  --user p0-agent

# Verify installation
sudo p0-ssh-agent status
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
orgId: "organization-name" # Your organization identifier
hostId: "machine-hostname" # Unique host identifier
tunnelHost: "wss://api.p0.app" # WebSocket URL (ws:// or wss://)

# Optional fields
hostname: "custom-hostname" # Override system hostname (optional)
keyPath: "/path/to/keys" # JWT key storage directory
environment: "production" # Environment identifier
heartbeatIntervalSeconds: 60 # Heartbeat interval in seconds (default: 60)
dryRun: false # Enable dry-run mode globally

# Machine labels (optional)
labels:
  - "type=production"
  - "region=us-west-2"
  - "team=infrastructure"
```
