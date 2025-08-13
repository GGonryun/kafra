# P0 SSH Agent Scripts

This package provides three core provisioning functions that implement user access management on Linux systems. These functions are based on AWS SSM document patterns and provide secure, auditable user provisioning capabilities.

## Overview

The scripts package implements three main provisioning operations:

1. **`ProvisionUser`** - Creates and manages user accounts (`provision_user.go`)
2. **`ProvisionAuthorizedKeys`** - Manages SSH authorized keys for users (`provision_keys.go`)
3. **`ProvisionSudo`** - Manages sudo access permissions (`provision_sudo.go`)

Each function supports both `grant` and `revoke` actions, enabling complete lifecycle management of user access.

## File Structure

- `types.go` - Data structures and constants
- `shared.go` - Common utility functions used across all provisioning operations
- `provision_user.go` - User account creation and management
- `provision_keys.go` - SSH authorized keys management
- `provision_sudo.go` - Sudo access management
- `README.md` - This documentation

## Data Structures

### ProvisioningRequest

All functions accept a `ProvisioningRequest` struct:

```go
type ProvisioningRequest struct {
    UserName  string `json:"userName"`   // Username (must match ^[a-z][-a-z0-9_]*$)
    Action    string `json:"action"`     // "grant" or "revoke"
    RequestID string `json:"requestId"`  // P0 access request identifier
    PublicKey string `json:"publicKey,omitempty"` // SSH public key (optional)
    Sudo      bool   `json:"sudo,omitempty"`      // Whether to grant sudo access
}
```

### ProvisioningResult

All functions return a `ProvisioningResult` struct:

```go
type ProvisioningResult struct {
    Success bool   `json:"success"` // Whether the operation succeeded
    Message string `json:"message"` // Success or informational message
    Error   string `json:"error,omitempty"` // Error message if operation failed
}
```

## Functions

### ProvisionUser(req ProvisioningRequest, logger *logrus.Logger) ProvisioningResult

**Purpose**: Creates and manages user accounts on the system.

**Grant Action**:
- Validates username format against `^[a-z][-a-z0-9_]*$`
- Finds next available UID in range 65536-90000
- Creates user account with home directory
- Uses `useradd`/`groupadd` or `adduser` depending on system
- Sets shell to `/bin/bash`

**Revoke Action**:
- Returns success (actual access removal handled by other functions)
- Does not delete user account for audit purposes

**Inputs**:
- `req.UserName`: Required username
- `req.Action`: "grant" or "revoke"
- `req.RequestID`: P0 request identifier for auditing

**Outputs**:
- Success: User created/managed successfully
- Error: Invalid username, UID exhaustion, or system command failure

### ProvisionAuthorizedKeys(req ProvisioningRequest, logger *logrus.Logger) ProvisioningResult

**Purpose**: Manages SSH authorized keys for user authentication.

**Grant Action**:
- Creates `~/.ssh/authorized_keys` if it doesn't exist
- Adds public key with RequestID comment for tracking
- Sets proper file permissions (600)
- Sets ownership to the target user

**Revoke Action**:
- Removes public key entries associated with the RequestID
- Uses sed pattern matching to remove key and comment blocks

**Inputs**:
- `req.UserName`: Target username
- `req.Action`: "grant" or "revoke"
- `req.RequestID`: P0 request identifier for tracking
- `req.PublicKey`: SSH public key (skipped if empty or "N/A")

**Outputs**:
- Success: Key added/removed successfully
- Error: User not found, file permission issues, or system command failure

### ProvisionSudo(req ProvisioningRequest, logger *logrus.Logger) ProvisioningResult

**Purpose**: Manages passwordless sudo access for users.

**Grant Action**:
- Creates `/etc/sudoers-p0` file with proper permissions (440)
- Adds NOPASSWD sudo rule for the user
- Ensures `#include sudoers-p0` line exists in `/etc/sudoers`
- Associates rule with RequestID for tracking

**Revoke Action**:
- Removes sudo rules associated with the RequestID from `/etc/sudoers-p0`
- Uses sed pattern matching to remove rule and comment blocks

**Inputs**:
- `req.UserName`: Target username
- `req.Action`: "grant" or "revoke"
- `req.RequestID`: P0 request identifier for tracking
- `req.Sudo`: Must be `true` for operation to proceed

**Outputs**:
- Success: Sudo access granted/revoked successfully  
- Error: File permission issues or system command failure

## Security Features

### Audit Trail
- All operations include RequestID comments for tracking
- Operations are logged with structured logging
- Failed operations are logged with detailed error information

### Validation
- Username format validation using regex patterns
- UID range enforcement (65536-90000) to avoid system conflicts
- File permission enforcement (600 for keys, 440 for sudoers)

### Privilege Management
- Uses `sudo` for privileged operations
- Separate sudoers file (`/etc/sudoers-p0`) for P0-managed rules
- Proper ownership management for user files

## Error Handling

Functions handle common error scenarios:
- Missing system commands (useradd, adduser, etc.)
- Permission denied errors
- File system errors
- Invalid input validation
- UID exhaustion

## Usage Example

```go
import (
    "github.com/sirupsen/logrus"
    "p0-ssh-agent/scripts"
)

req := scripts.ProvisioningRequest{
    UserName:  "john-doe",
    Action:    "grant",
    RequestID: "req123",
    PublicKey: "ssh-rsa AAAAB3NzaC1yc2E...",
    Sudo:      true,
}

logger := logrus.New()

// Create user
userResult := scripts.ProvisionUser(req, logger)
if !userResult.Success {
    log.Error("Failed to provision user:", userResult.Error)
    return
}

// Add SSH key
keyResult := scripts.ProvisionAuthorizedKeys(req, logger)
if !keyResult.Success {
    log.Error("Failed to provision keys:", keyResult.Error)
    return
}

// Grant sudo access
sudoResult := scripts.ProvisionSudo(req, logger)
if !sudoResult.Success {
    log.Error("Failed to provision sudo:", sudoResult.Error)
    return
}
```

## System Requirements

- Linux operating system
- `sudo` access for the agent
- One of: `useradd`/`groupadd` or `adduser` commands
- Standard shell utilities: `mkdir`, `touch`, `chmod`, `chown`, `grep`, `sed`, `tee`

## Shared Utilities

The `shared.go` file contains common utility functions used across all provisioning operations:

- **`isValidUsername()`** - Validates username format against P0 requirements
- **`findNextAvailableUID()`** - Finds available UID in range 65536-90000
- **`commandExists()`** - Checks if system commands are available
- **`ensureContentInFile()`** - Adds content to files with proper permissions
- **`removeContentFromFile()`** - Removes content based on RequestID tracking
- **`ensureLineInFile()`** - Ensures a line exists in a file

These utilities provide consistent behavior and error handling across all provisioning functions.

## File Locations

- SSH keys: `~/.ssh/authorized_keys`
- Sudo rules: `/etc/sudoers-p0`
- Main sudoers: `/etc/sudoers` (for include directive)