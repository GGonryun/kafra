# P0 SSH Agent

A lightweight Go client that connects to the P0 backend infrastructure via WebSocket, authenticates using JWT tokens with ECDSA P-384 keys, and logs incoming requests for monitoring and debugging purposes.

## Features

- **JWT Authentication**: ES384 algorithm with ECDSA P-384 private keys
- **WebSocket Communication**: Secure WebSocket connections with JSON-RPC 2.0 protocol
- **Request Logging**: Logs all incoming requests from P0 backend for monitoring
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
./bin/p0-ssh-agent keygen --path ~/.p0/keys
```

The keygen command will:
- Generate ECDSA P-384 private/public keypair
- Save `jwk.private.json` and `jwk.public.json`
- Display the public key for backend registration
- Protect against accidental overwriting (use `--force` to override)

### 3. Register Public Key

Copy the public key output from keygen and register it with your P0 backend infrastructure.

### 4. Start the Agent

```bash
./bin/p0-ssh-agent start --client-id your-client-id
```

## Configuration Options

| Flag | Description | Default |
|------|-------------|---------|
| `--client-id` | Client identifier (required) | - |
| `--tunnel-host` | P0 backend host | `localhost` |
| `--tunnel-port` | P0 backend port | `8080` |
| `--tunnel-path` | WebSocket endpoint path | `/` |
| `--insecure` | Use `ws://` instead of `wss://` | `false` |
| `--jwk-path` | Directory containing JWT key files | `.` |
| `--verbose` | Enable debug logging | `false` |

## Usage Examples

```bash
# Generate keys first
./bin/p0-ssh-agent keygen --path ~/.p0/keys

# Basic agent startup
./bin/p0-ssh-agent start --client-id client-123

# Connect to remote backend with custom path
./bin/p0-ssh-agent start --client-id client-123 \
  --tunnel-host backend.example.com \
  --tunnel-port 443 \
  --tunnel-path /socket

# Debug connection issues
./bin/p0-ssh-agent start --client-id client-123 --verbose

# Use custom key directory
./bin/p0-ssh-agent start --client-id client-123 --jwk-path ~/.p0/keys

# Force regenerate keys (dangerous!)
./bin/p0-ssh-agent keygen --path ~/.p0/keys --force

# Show help for specific commands
./bin/p0-ssh-agent help
./bin/p0-ssh-agent start --help
./bin/p0-ssh-agent keygen --help
```

## Architecture

### Authentication Flow
1. Agent loads ECDSA P-384 private key from `jwk.private.json`
2. Creates JWT token with ES384 algorithm and client ID
3. Establishes WebSocket connection with `Authorization: Bearer <token>` header
4. Sends `setClientId` RPC call to register with backend

### Request Handling
- Receives `call` method requests via JSON-RPC 2.0
- Logs request details (method, path, headers, parameters)
- Returns success response to acknowledge receipt
- Filters sensitive headers from logs (e.g., authorization)

### Connection Management
- Automatic reconnection with exponential backoff (1s to 30s)
- Graceful shutdown on SIGINT/SIGTERM
- Connection status monitoring and detailed error reporting

## Available Commands

The p0-ssh-agent binary includes multiple subcommands:

### Commands
- `start` - Start the WebSocket proxy agent
- `keygen` - Generate JWT keypair for authentication  
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
./bin/p0-ssh-agent keygen --path ~/.p0/keys

# Then run agent
./bin/p0-ssh-agent start --client-id your-id --jwk-path ~/.p0/keys
```

**Permission denied errors:**
```bash
# Create secure key directory
mkdir -p ~/.p0/keys
chmod 700 ~/.p0/keys

# Generate keys there
./bin/p0-ssh-agent keygen --path ~/.p0/keys
```

### Debug Logging

Enable verbose logging to see detailed connection information:

```bash
./bin/p0-ssh-agent start --client-id your-id --verbose
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
p0-ssh-agent start --client-id $(hostname) --tunnel-host production.p0.dev
```

### Systemd Service Example

Create `/etc/systemd/system/p0-ssh-agent.service`:

```ini
[Unit]
Description=P0 SSH Agent
After=network.target

[Service]
Type=simple
User=p0-agent
ExecStart=/usr/local/bin/p0-ssh-agent start --client-id %H --tunnel-host production.p0.dev --jwk-path /etc/p0/keys
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

## Security Notes

- Private keys are stored locally and never transmitted
- JWT tokens are short-lived and created per connection
- Authorization headers are filtered from request logs
- Key files should have restrictive permissions (600/700)
- Use `wss://` (secure WebSocket) for production deployments