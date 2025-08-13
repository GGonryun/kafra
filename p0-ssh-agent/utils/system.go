package utils

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
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
	hostname, err := os.Hostname()
	if err != nil {
		logger.WithError(err).Warn("Failed to get hostname, using fallback")
		return "unknown-host"
	}
	logger.WithField("hostname", hostname).Debug("Retrieved hostname")
	return hostname
}

// GetPublicIP attempts to get the public IP address using multiple services
func GetPublicIP(logger *logrus.Logger) string {
	client := &http.Client{Timeout: httpTimeout}
	
	for _, service := range publicIPServices {
		resp, err := client.Get(service)
		if err != nil {
			logger.WithError(err).WithField("service", service).Debug("Failed to get public IP from service")
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			buf := make([]byte, maxResponseSize)
			n, err := resp.Body.Read(buf)
			if err != nil && n == 0 {
				continue
			}
			ip := strings.TrimSpace(string(buf[:n]))
			if isValidIP(ip) {
				logger.WithField("publicIP", ip).Debug("Retrieved public IP")
				return ip
			}
		}
	}

	logger.Warn("Failed to get public IP, using fallback")
	return "unknown"
}

// GetMachineFingerprint creates a machine fingerprint using SSH host keys
func GetMachineFingerprint(logger *logrus.Logger) string {
	// Try to get SSH host key fingerprint
	for _, keyPath := range sshHostKeyPaths {
		if fingerprint := getSSHKeyFingerprint(keyPath, logger); fingerprint != "" {
			logger.WithField("fingerprint", fingerprint).Debug("Generated SSH-based machine fingerprint")
			return fingerprint
		}
	}
	
	// Fallback: generate fingerprint from hostname and MAC addresses
	return getFallbackFingerprint(logger)
}

// GetMachinePublicKey returns the SSH host public key
func GetMachinePublicKey(logger *logrus.Logger) string {
	for _, keyPath := range sshHostKeyPaths {
		if _, err := os.Stat(keyPath); err == nil {
			data, err := os.ReadFile(keyPath)
			if err != nil {
				logger.WithError(err).WithField("keyPath", keyPath).Debug("Failed to read SSH host key")
				continue
			}
			
			publicKey := strings.TrimSpace(string(data))
			if publicKey != "" {
				logger.WithField("keyPath", keyPath).Debug("Retrieved SSH host public key")
				return publicKey
			}
		}
	}
	
	// Fallback: generate a deterministic key based on machine info
	return getFallbackPublicKey(logger)
}

// getSSHKeyFingerprint extracts SHA256 fingerprint from SSH host key
func getSSHKeyFingerprint(keyPath string, logger *logrus.Logger) string {
	if _, err := os.Stat(keyPath); err != nil {
		return ""
	}
	
	// Use ssh-keygen to get the fingerprint
	cmd := exec.Command("ssh-keygen", "-l", "-f", keyPath, "-E", "sha256")
	output, err := cmd.Output()
	if err != nil {
		logger.WithError(err).WithField("keyPath", keyPath).Debug("Failed to get SSH key fingerprint")
		return ""
	}
	
	// Parse output: "2048 SHA256:fingerprint host (RSA)"
	parts := strings.Fields(string(output))
	for _, part := range parts {
		if strings.HasPrefix(part, "SHA256:") {
			return strings.TrimPrefix(part, "SHA256:")
		}
	}
	
	return ""
}

// getFallbackFingerprint creates a fingerprint from hostname and MAC addresses
func getFallbackFingerprint(logger *logrus.Logger) string {
	hostname, _ := os.Hostname()
	
	// Get MAC addresses
	interfaces, err := net.Interfaces()
	var macAddresses []string
	if err == nil {
		for _, iface := range interfaces {
			if iface.HardwareAddr != nil && len(iface.HardwareAddr) > 0 {
				// Skip loopback and virtual interfaces
				if iface.Flags&net.FlagLoopback == 0 && !strings.HasPrefix(iface.Name, "docker") {
					macAddresses = append(macAddresses, iface.HardwareAddr.String())
				}
			}
		}
	}

	// Create fingerprint from available data
	data := hostname + strings.Join(macAddresses, "")
	if data == "" {
		data = "fallback-fingerprint"
	}

	hash := sha256.Sum256([]byte(data))
	fingerprint := fmt.Sprintf("%x", hash)[:32] // Use first 32 chars
	
	logger.WithField("fingerprint", fingerprint).Debug("Generated fallback machine fingerprint")
	return fingerprint
}

// getFallbackPublicKey generates a deterministic public key based on machine info
func getFallbackPublicKey(logger *logrus.Logger) string {
	hostname, _ := os.Hostname()
	data := "machine-public-key-" + hostname
	hash := sha256.Sum256([]byte(data))
	key := base64.StdEncoding.EncodeToString(hash[:])
	
	logger.WithField("publicKey", key[:16]+"...").Debug("Generated fallback public key")
	return key
}

// isValidIP validates if a string is a valid IP address
func isValidIP(ip string) bool {
	return net.ParseIP(ip) != nil
}

// GenerateRegistrationCode creates a registration code combining system information
func GenerateRegistrationCode(hostname, publicIP, fingerprint, publicKey string) string {
	parts := []string{hostname, publicIP, fingerprint, publicKey}
	return strings.Join(parts, ",")
}