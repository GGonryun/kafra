#!/bin/bash

# P0 SSH Agent Bootstrap Script
# This script sets up the basic files needed for P0 SSH Agent installation

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
BINARY_NAME="p0-ssh-agent"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/p0-ssh-agent"
CONFIG_FILE="$CONFIG_DIR/config.yaml"

echo -e "${BLUE}🚀 P0 SSH Agent Bootstrap Script${NC}"
echo "=================================================="

# Check if running as root
if [[ $EUID -eq 0 ]]; then
   echo -e "${RED}❌ This script should not be run as root${NC}"
   echo "Please run as a regular user with sudo privileges"
   exit 1
fi

# Check if binary exists in current directory
if [[ ! -f "$BINARY_NAME" ]]; then
    echo -e "${RED}❌ Binary '$BINARY_NAME' not found in current directory${NC}"
    echo "Please build the binary first with: go build -o $BINARY_NAME cmd/main.go"
    exit 1
fi

echo -e "${YELLOW}📦 Installing P0 SSH Agent binary...${NC}"

# Copy binary to system location
sudo cp "$BINARY_NAME" "$INSTALL_DIR/"
sudo chmod +x "$INSTALL_DIR/$BINARY_NAME"

echo -e "${GREEN}✅ Binary installed to $INSTALL_DIR/$BINARY_NAME${NC}"

echo -e "${YELLOW}📁 Creating configuration directory...${NC}"

# Create config directory
sudo mkdir -p "$CONFIG_DIR"

echo -e "${YELLOW}📝 Creating default configuration file...${NC}"

# Create default config file
sudo tee "$CONFIG_FILE" > /dev/null << 'EOF'
# P0 SSH Agent Configuration File
# Please update these values for your environment

# Required: Organization and host identification
orgId: "my-organization"           # Replace with your organization ID
hostId: "hostname-goes-here"       # Replace with unique host identifier

# Required: P0 backend connection
tunnelHost: "wss://p0.example.com/websocket"  # Replace with your P0 backend URL

# File paths
keyPath: "/etc/p0-ssh-agent/keys"    # JWT key storage directory
logPath: "/var/log/p0-ssh-agent"     # Log file directory

# Optional: Machine labels for identification
labels:
  - "environment=production"
  - "team=infrastructure"
  - "region=us-west-2"

# Optional: Advanced settings
environment: "production"
tunnelTimeoutSeconds: 30
version: "1.0"
EOF

# Set proper permissions on config file
sudo chmod 644 "$CONFIG_FILE"

echo -e "${GREEN}✅ Configuration file created at $CONFIG_FILE${NC}"

echo ""
echo -e "${BLUE}📋 Next Steps:${NC}"
echo "=============="
echo -e "1. ${YELLOW}Edit the configuration file:${NC}"
echo "   sudo vi $CONFIG_FILE"
echo ""
echo -e "2. ${YELLOW}Update the following required fields:${NC}"
echo "   - orgId: Your organization identifier"
echo "   - hostId: Unique identifier for this machine"
echo "   - tunnelHost: Your P0 backend WebSocket URL"
echo ""
echo -e "3. ${YELLOW}Run the installation:${NC}"
echo "   sudo $BINARY_NAME install"
echo ""
echo -e "${GREEN}🎉 Bootstrap complete!${NC}"
echo ""
echo -e "${BLUE}ℹ️  The install command will:${NC}"
echo "   • Validate your configuration"
echo "   • Create service user and directories"
echo "   • Generate JWT keys"
echo "   • Register with P0 backend"
echo "   • Create and start systemd service"