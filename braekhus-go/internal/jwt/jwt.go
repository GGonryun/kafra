package jwt

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/sirupsen/logrus"
)

const (
	// ES384 algorithm for JWT signing
	Algorithm = "ES384"
	
	// Key files
	PrivateKeyFile = "jwk.private.json"
	PublicKeyFile  = "jwk.public.json"
)

// JWK represents a JSON Web Key
type JWK struct {
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
	D   string `json:"d,omitempty"` // Only in private key
}

// Manager handles JWT operations
type Manager struct {
	logger     *logrus.Logger
	privateKey *ecdsa.PrivateKey
}

// NewManager creates a new JWT manager
func NewManager(logger *logrus.Logger) *Manager {
	return &Manager{
		logger: logger,
	}
}

// EnsureKey loads or generates a key pair at the given path
func (m *Manager) EnsureKey(path string) error {
	privateKeyPath := filepath.Join(path, PrivateKeyFile)
	
	// Try to load existing key
	if key, err := m.loadKey(privateKeyPath); err == nil {
		m.privateKey = key
		return nil
	}
	
	// Generate new key pair
	return m.generateKeyPair(path)
}

// loadKey loads a private key from file
func (m *Manager) loadKey(path string) (*ecdsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	
	var jwk JWK
	if err := json.Unmarshal(data, &jwk); err != nil {
		return nil, err
	}
	
	// Convert JWK to ECDSA private key
	// This is a simplified version - in production you'd want proper JWK parsing
	return nil, fmt.Errorf("JWK to ECDSA conversion not implemented in this simplified version")
}

// generateKeyPair generates a new ES384 key pair
func (m *Manager) generateKeyPair(path string) error {
	// Generate ECDSA key pair for ES384 (P-384 curve)
	privateKey, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate key pair: %w", err)
	}
	
	m.privateKey = privateKey
	
	// Save private key as PEM (simplified - in production use JWK format)
	privateKeyPath := filepath.Join(path, PrivateKeyFile)
	if err := m.savePrivateKeyPEM(privateKeyPath, privateKey); err != nil {
		return fmt.Errorf("failed to save private key: %w", err)
	}
	
	// Set proper permissions
	if err := os.Chmod(privateKeyPath, 0400); err != nil {
		return fmt.Errorf("failed to set key permissions: %w", err)
	}
	
	// Save public key as PEM (simplified)
	publicKeyPath := filepath.Join(path, PublicKeyFile)
	if err := m.savePublicKeyPEM(publicKeyPath, &privateKey.PublicKey); err != nil {
		return fmt.Errorf("failed to save public key: %w", err)
	}
	
	m.logger.Info("Generated new ES384 key pair")
	return nil
}

// savePrivateKeyPEM saves a private key in PEM format
func (m *Manager) savePrivateKeyPEM(path string, key *ecdsa.PrivateKey) error {
	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return err
	}
	
	keyPEM := &pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyBytes,
	}
	
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	
	return pem.Encode(file, keyPEM)
}

// savePublicKeyPEM saves a public key in PEM format
func (m *Manager) savePublicKeyPEM(path string, key *ecdsa.PublicKey) error {
	keyBytes, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return err
	}
	
	keyPEM := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: keyBytes,
	}
	
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	
	return pem.Encode(file, keyPEM)
}

// CreateJWT creates a signed JWT token
func (m *Manager) CreateJWT(clientID string) (string, error) {
	if m.privateKey == nil {
		return "", fmt.Errorf("private key not loaded")
	}
	
	now := time.Now()
	claims := jwt.MapClaims{
		"tunnel-id": "my-tunnel-id",
		"iat":       now.Unix(),
		"exp":       now.Add(7 * 24 * time.Hour).Unix(), // One week
		"aud":       "p0.dev",
		"sub":       clientID,
		"iss":       "kd-client",
	}
	
	token := jwt.NewWithClaims(jwt.SigningMethodES384, claims)
	return token.SignedString(m.privateKey)
}