# Braekhus Go Client

A Go implementation of the Braekhus reverse hole-punching proxy client. This client connects to a Braekhus server using WebSockets and JSON-RPC, authenticates with JWT tokens, and forwards HTTP requests to a target service.

## Features

- **WebSocket Connection**: Establishes secure WebSocket connections to the Braekhus server
- **JWT Authentication**: Uses ES384 JWT tokens signed with ECDSA private keys for authentication
- **JSON-RPC Communication**: Implements JSON-RPC 2.0 for bidirectional communication
- **Automatic Reconnection**: Includes exponential backoff retry mechanism for connection resilience
- **Machine Registration**: Each client identifies itself with a unique client ID and JWT
- **Request Forwarding**: Placeholder implementation for forwarding HTTP requests to target services

## Architecture

The Go client is structured as follows:

- **`cmd/client/`**: CLI application entry point
- **`internal/client/`**: Main client implementation with WebSocket and RPC handling
- **`internal/jwt/`**: JWT creation and signing with ES384 algorithm
- **`internal/rpc/`**: JSON-RPC 2.0 client/server implementation
- **`internal/backoff/`**: Exponential backoff retry mechanism
- **`internal/config/`**: Configuration management
- **`pkg/types/`**: Shared type definitions

## Installation

1. Clone the repository
2. Navigate to the `braekhus-go` directory
3. Install dependencies:
   ```bash
   go mod tidy
   ```
4. Build the client:
   ```bash
   go build -o braekhus-client ./cmd/client
   ```

## Usage

### Command Line Options

```bash
./braekhus-client [flags]

Flags:
  --target-url string    Target URL to forward requests to (required)
  --client-id string     Client identifier (required)
  --tunnel-host string   Tunnel server host (default "localhost")
  --tunnel-port int      Tunnel server port (default 8080)
  --insecure            Use insecure WebSocket connection (ws instead of wss)
  --jwk-path string     Path to store JWT key files (default ".")
  -v, --verbose         Enable verbose logging
  -h, --help            help for braekhus-client
```

### Example Usage

```bash
# Basic usage
./braekhus-client \
  --target-url http://localhost:8000 \
  --client-id myClientId \
  --tunnel-host localhost \
  --tunnel-port 8080 \
  --insecure

# With verbose logging
./braekhus-client \
  --target-url http://localhost:8000 \
  --client-id myClientId \
  --tunnel-host production.example.com \
  --tunnel-port 443 \
  --jwk-path /etc/braekhus/keys \
  --verbose
```

## JWT Key Management

The client automatically generates and manages JWT key pairs:

- **Private Key**: `{jwk-path}/jwk.private.json` (permissions: 400)
- **Public Key**: `{jwk-path}/jwk.public.json`

The keys use the ES384 algorithm (ECDSA with P-384 curve and SHA-384 hash).

## Configuration

Configuration can be provided via:

1. Command line flags
2. Environment variables (prefixed with `BRAEKHUS_`)
3. Configuration files (`config.yaml`)

Environment variable examples:
```bash
export BRAEKHUS_TARGET_URL=http://localhost:8000
export BRAEKHUS_CLIENT_ID=myClientId
export BRAEKHUS_TUNNEL_HOST=localhost
export BRAEKHUS_TUNNEL_PORT=8080
```

## Implementation Details

### WebSocket Connection

The client establishes a WebSocket connection to the server with:
- JWT token in the `Authorization` header
- Automatic reconnection with exponential backoff
- Graceful shutdown handling

### JSON-RPC Methods

The client implements the following RPC methods:

- **`setClientId`**: Registers the client ID with the server
- **`call`**: Placeholder for handling forwarded HTTP requests

### Call Method Placeholder

The `call` method currently returns a placeholder response. To implement actual request forwarding, you need to:

1. Parse the `ForwardedRequest` from the RPC parameters
2. Make an HTTP request to the target URL
3. Apply any necessary transformations (e.g., header filtering)
4. Return a `ForwardedResponse` with the result

Example implementation structure:
```go
func (c *Client) handleCallMethod(params interface{}) (interface{}, error) {
    // Parse ForwardedRequest
    var req types.ForwardedRequest
    // ... parsing logic ...
    
    // Make HTTP request to target
    response, err := http.DefaultClient.Do(httpReq)
    // ... error handling ...
    
    // Return ForwardedResponse
    return types.ForwardedResponse{
        Headers:    response.Header,
        Status:     response.StatusCode,
        StatusText: response.Status,
        Data:       responseBody,
    }, nil
}
```

## Dependencies

- **github.com/golang-jwt/jwt/v5**: JWT token creation and signing
- **github.com/gorilla/websocket**: WebSocket client implementation
- **github.com/sirupsen/logrus**: Structured logging
- **github.com/spf13/cobra**: CLI framework
- **github.com/spf13/viper**: Configuration management

## Logging

The client uses structured logging with configurable log levels:
- Default: `INFO` level
- Verbose mode (`-v`): `DEBUG` level

Log messages include contextual information such as connection status, RPC calls, and error details.

## Security Considerations

- JWT tokens are signed with ECDSA private keys using ES384
- Private keys are stored with restrictive permissions (400)
- WebSocket connections support TLS encryption
- Client authentication prevents unauthorized access