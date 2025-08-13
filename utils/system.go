package utils

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"p0-ssh-agent/internal/config"
	"p0-ssh-agent/internal/jwt"
	"p0-ssh-agent/types"
)

const (
	httpTimeout     = 5 * time.Second
	maxResponseSize = 64
)

var (
	// SSH host key paths to check for fingerprinting
	sshHostKeyPaths = []string{
		"/etc/ssh/ssh_host_ed25519_key.pub",
		"/etc/ssh/ssh_host_rsa_key.pub",
		"/etc/ssh/ssh_host_ecdsa_key.pub",
	}

	// Public IP services to try
	publicIPServices = []string{
		"https://api.ipify.org",
		"https://checkip.amazonaws.com",
		"https://icanhazip.com",
	}
)

// GetHostname returns the system hostname
func GetHostname(logger *logrus.Logger) string {
	logger.Debug("Starting hostname collection...")

	hostname, err := os.Hostname()
	if err != nil {
		logger.WithError(err).Warn("Failed to get system hostname from os.Hostname(), using fallback")
		logger.Info("🏠 Hostname source: fallback (os.Hostname() failed)")
		return "unknown-host"
	}

	logger.WithField("hostname", hostname).Debug("Successfully retrieved hostname from os.Hostname()")
	logger.WithField("hostname", hostname).Info("🏠 Hostname source: system (os.Hostname())")
	return hostname
}

// GetPublicIP attempts to get the public IP address using multiple services
func GetPublicIP(logger *logrus.Logger) string {
	logger.Debug("Starting public IP discovery...")
	logger.WithField("services", publicIPServices).Debug("Trying public IP services in order")

	client := &http.Client{Timeout: httpTimeout}

	for i, service := range publicIPServices {
		logger.WithFields(logrus.Fields{
			"service": service,
			"attempt": i + 1,
			"total":   len(publicIPServices),
		}).Debug("Attempting to get public IP from service")

		resp, err := client.Get(service)
		if err != nil {
			logger.WithError(err).WithField("service", service).Warn("Failed to connect to public IP service")
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			logger.WithFields(logrus.Fields{
				"service":    service,
				"statusCode": resp.StatusCode,
			}).Warn("Public IP service returned non-200 status")
			continue
		}

		buf := make([]byte, maxResponseSize)
		n, err := resp.Body.Read(buf)
		if err != nil && n == 0 {
			logger.WithError(err).WithField("service", service).Warn("Failed to read response from public IP service")
			continue
		}

		ip := strings.TrimSpace(string(buf[:n]))
		logger.WithFields(logrus.Fields{
			"service": service,
			"rawIP":   ip,
		}).Debug("Received IP response from service")

		if isValidIP(ip) {
			logger.WithFields(logrus.Fields{
				"accessIp": ip,
				"service":  service,
			}).Info("🌐 Public IP source: external service")
			return ip
		} else {
			logger.WithFields(logrus.Fields{
				"service":   service,
				"invalidIP": ip,
			}).Warn("Received invalid IP address from service")
		}
	}

	logger.Warn("All public IP services failed or returned invalid IPs, using fallback")
	logger.Info("🌐 Public IP source: fallback (all services failed)")
	return "unknown"
}

// GetMachineFingerprint creates a machine fingerprint using SSH host keys
func GetMachineFingerprint(logger *logrus.Logger) string {
	logger.Debug("Starting machine fingerprint generation...")
	logger.WithField("sshKeyPaths", sshHostKeyPaths).Debug("Checking SSH host key paths for fingerprinting")

	// Try to get SSH host key fingerprint
	for i, keyPath := range sshHostKeyPaths {
		logger.WithFields(logrus.Fields{
			"keyPath": keyPath,
			"attempt": i + 1,
			"total":   len(sshHostKeyPaths),
		}).Debug("Checking SSH host key path")

		if fingerprint := getSSHKeyFingerprint(keyPath, logger); fingerprint != "" {
			logger.WithFields(logrus.Fields{
				"fingerprint": fingerprint,
				"keyPath":     keyPath,
			}).Info("🔑 Fingerprint source: SSH host key")
			return fingerprint
		}
	}

	logger.Warn("No SSH host keys found or usable, falling back to system-based fingerprint")
	// Fallback: generate fingerprint from hostname and MAC addresses
	return getFallbackFingerprint(logger)
}

// GetMachinePublicKey returns the SSH host public key
func GetMachinePublicKey(logger *logrus.Logger) string {
	logger.Debug("Starting machine public key collection...")
	logger.WithField("sshKeyPaths", sshHostKeyPaths).Debug("Checking SSH host key paths for public key")

	for i, keyPath := range sshHostKeyPaths {
		logger.WithFields(logrus.Fields{
			"keyPath": keyPath,
			"attempt": i + 1,
			"total":   len(sshHostKeyPaths),
		}).Debug("Checking SSH host public key path")

		if _, err := os.Stat(keyPath); err == nil {
			logger.WithField("keyPath", keyPath).Debug("SSH host key file exists, reading...")

			data, err := os.ReadFile(keyPath)
			if err != nil {
				logger.WithError(err).WithField("keyPath", keyPath).Warn("Failed to read SSH host key file")
				continue
			}

			publicKey := strings.TrimSpace(string(data))
			if publicKey != "" {
				logger.WithFields(logrus.Fields{
					"keyPath":   keyPath,
					"keyLength": len(publicKey),
					"keyType":   strings.Fields(publicKey)[0], // Usually ssh-rsa, ssh-ed25519, etc.
				}).Info("🔐 Public key source: SSH host key file")
				return publicKey
			} else {
				logger.WithField("keyPath", keyPath).Warn("SSH host key file is empty")
			}
		} else {
			logger.WithFields(logrus.Fields{
				"keyPath": keyPath,
				"error":   err.Error(),
			}).Debug("SSH host key file does not exist or is not accessible")
		}
	}

	logger.Warn("No SSH host public keys found or readable, falling back to generated key")
	// Fallback: generate a deterministic key based on machine info
	return getFallbackPublicKey(logger)
}

// getSSHKeyFingerprint extracts SHA256 fingerprint from SSH host key
func getSSHKeyFingerprint(keyPath string, logger *logrus.Logger) string {
	if _, err := os.Stat(keyPath); err != nil {
		logger.WithFields(logrus.Fields{
			"keyPath": keyPath,
			"error":   err.Error(),
		}).Debug("SSH key file does not exist")
		return ""
	}

	logger.WithField("keyPath", keyPath).Debug("SSH key file exists, extracting fingerprint with ssh-keygen")

	// Use ssh-keygen to get the fingerprint
	cmd := exec.Command("ssh-keygen", "-l", "-f", keyPath, "-E", "sha256")
	output, err := cmd.Output()
	if err != nil {
		logger.WithError(err).WithFields(logrus.Fields{
			"keyPath": keyPath,
			"command": "ssh-keygen -l -f " + keyPath + " -E sha256",
		}).Debug("ssh-keygen command failed")
		return ""
	}

	outputStr := string(output)
	logger.WithFields(logrus.Fields{
		"keyPath":      keyPath,
		"sshKeygenOut": outputStr,
	}).Debug("ssh-keygen output received")

	// Parse output: "2048 SHA256:fingerprint host (RSA)"
	parts := strings.Fields(outputStr)
	for _, part := range parts {
		if strings.HasPrefix(part, "SHA256:") {
			// Keep the SHA256: prefix in the fingerprint
			logger.WithFields(logrus.Fields{
				"keyPath":     keyPath,
				"fingerprint": part,
			}).Debug("Successfully extracted SHA256 fingerprint from SSH key")
			return part
		}
	}

	logger.WithFields(logrus.Fields{
		"keyPath": keyPath,
		"output":  outputStr,
	}).Warn("Could not parse SHA256 fingerprint from ssh-keygen output")
	return ""
}

// getFallbackFingerprint creates a fingerprint from hostname and MAC addresses
func getFallbackFingerprint(logger *logrus.Logger) string {
	logger.Debug("Generating fallback fingerprint from hostname and MAC addresses...")

	hostname, err := os.Hostname()
	if err != nil {
		logger.WithError(err).Debug("Failed to get hostname for fallback fingerprint")
		hostname = "unknown"
	}
	logger.WithField("hostname", hostname).Debug("Using hostname for fallback fingerprint")

	// Get MAC addresses
	interfaces, err := net.Interfaces()
	var macAddresses []string
	var skippedInterfaces []string

	if err == nil {
		logger.Debug("Collecting MAC addresses from network interfaces...")
		for _, iface := range interfaces {
			if iface.HardwareAddr != nil && len(iface.HardwareAddr) > 0 {
				// Skip loopback and virtual interfaces
				if iface.Flags&net.FlagLoopback == 0 && !strings.HasPrefix(iface.Name, "docker") {
					macAddresses = append(macAddresses, iface.HardwareAddr.String())
					logger.WithFields(logrus.Fields{
						"interface": iface.Name,
						"mac":       iface.HardwareAddr.String(),
					}).Debug("Added MAC address for fingerprint")
				} else {
					skippedInterfaces = append(skippedInterfaces, iface.Name)
				}
			}
		}

		if len(skippedInterfaces) > 0 {
			logger.WithField("skippedInterfaces", skippedInterfaces).Debug("Skipped loopback/virtual interfaces")
		}

		logger.WithFields(logrus.Fields{
			"macCount": len(macAddresses),
			"macList":  macAddresses,
		}).Debug("Collected MAC addresses for fingerprint")
	} else {
		logger.WithError(err).Warn("Failed to get network interfaces for fallback fingerprint")
	}

	// Create fingerprint from available data
	data := hostname + strings.Join(macAddresses, "")
	if data == "" {
		logger.Warn("No hostname or MAC addresses available, using hardcoded fallback")
		data = "fallback-fingerprint"
	}

	hash := sha256.Sum256([]byte(data))
	hashString := fmt.Sprintf("%x", hash)[:32] // Use first 32 chars
	fingerprint := "SHA256:" + hashString

	logger.WithFields(logrus.Fields{
		"fingerprint": fingerprint,
		"sourceData":  fmt.Sprintf("hostname:%s + %d MAC addresses", hostname, len(macAddresses)),
	}).Info("🔑 Fingerprint source: system fallback (hostname + MAC addresses)")

	return fingerprint
}

// getFallbackPublicKey generates a deterministic public key based on machine info
func getFallbackPublicKey(logger *logrus.Logger) string {
	logger.Debug("Generating fallback public key from machine information...")

	hostname, err := os.Hostname()
	if err != nil {
		logger.WithError(err).Debug("Failed to get hostname for fallback public key")
		hostname = "unknown"
	}

	data := "machine-public-key-" + hostname
	hash := sha256.Sum256([]byte(data))
	key := base64.StdEncoding.EncodeToString(hash[:])

	logger.WithFields(logrus.Fields{
		"hostname":   hostname,
		"keyPreview": key[:16] + "...",
		"keyLength":  len(key),
		"sourceData": data,
	}).Info("🔐 Public key source: generated fallback (hostname-based)")

	return key
}

// isValidIP validates if a string is a valid IP address
func isValidIP(ip string) bool {
	return net.ParseIP(ip) != nil
}

// GenerateRegistrationCode creates a registration code combining system information
func GenerateRegistrationCode(hostname, accessIP, fingerprint, publicKey string) string {
	parts := []string{hostname, accessIP, fingerprint, publicKey}
	return strings.Join(parts, ",")
}

// CreateRegistrationRequest creates a complete registration request with system information
func CreateRegistrationRequest(configPath string, logger *logrus.Logger) (*types.RegistrationRequest, error) {
	logger.Debug("Creating registration request...")

	// Load configuration
	cfg, err := config.LoadWithOverrides(configPath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Collect system information using utils functions
	hostname := GetHostname(logger)
	accessIp := GetPublicIP(logger)
	fingerprint := GetMachineFingerprint(logger)
	fingerprintPublicKey := GetMachinePublicKey(logger)

	// Load JWK public key
	jwkPublicKey, err := GetJWKPublicKey(cfg.KeyPath, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to load JWK public key: %w", err)
	}

	// Create registration request
	request := &types.RegistrationRequest{
		HostID:               cfg.HostID,
		ClientID:             cfg.GetClientID(),
		Hostname:             hostname,
		AccessIP:             accessIp,
		Fingerprint:          fingerprint,
		FingerprintPublicKey: fingerprintPublicKey,
		JWKPublicKey:         jwkPublicKey,
		EnvironmentID:        cfg.Environment,
		OrgID:                cfg.OrgID,
		Labels:               cfg.Labels,
		Timestamp:            time.Now().UTC().Format(time.RFC3339),
	}

	logger.Debug("Registration request created successfully")
	return request, nil
}

// GenerateRegistrationRequestCode creates a base64-encoded registration request
func GenerateRegistrationRequestCode(configPath string, logger *logrus.Logger) (string, error) {
	request, err := CreateRegistrationRequest(configPath, logger)
	if err != nil {
		return "", err
	}

	// Convert to JSON and base64 encode
	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal registration request: %w", err)
	}

	encodedRequest := base64.StdEncoding.EncodeToString(jsonData)
	logger.Debug("Registration code generated successfully")

	return encodedRequest, nil
}

// GetJWKPublicKey loads and returns the JWK public key
func GetJWKPublicKey(keyPath string, logger *logrus.Logger) (map[string]string, error) {
	publicKeyPath := filepath.Join(keyPath, jwt.PublicKeyFile)

	data, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key file: %w", err)
	}

	var jwk map[string]interface{}
	if err := json.Unmarshal(data, &jwk); err != nil {
		return nil, fmt.Errorf("failed to parse JWK: %w", err)
	}

	// Convert to string map
	result := make(map[string]string)
	for k, v := range jwk {
		if str, ok := v.(string); ok {
			result[k] = str
		}
	}

	logger.WithField("keyPath", publicKeyPath).Debug("Loaded JWK public key")
	return result, nil
}
