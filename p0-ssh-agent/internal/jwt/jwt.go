package jwt

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-jose/go-jose/v3"
	"github.com/go-jose/go-jose/v3/jwt"
	"github.com/sirupsen/logrus"
)

const (
	Algorithm = "ES384"

	PrivateKeyFile = "jwk.private.json"
	PublicKeyFile  = "jwk.public.json"
)

type CustomClaims struct {
	TunnelID string `json:"tunnel-id"`
	jwt.Claims
}

type Manager struct {
	logger     *logrus.Logger
	privateJWK jose.JSONWebKey
	publicJWK  jose.JSONWebKey
	signer     jose.Signer
}

func NewManager(logger *logrus.Logger) *Manager {
	return &Manager{
		logger: logger,
	}
}

func (m *Manager) LoadKey(path string) error {
	privateKeyPath := filepath.Join(path, PrivateKeyFile)
	publicKeyPath := filepath.Join(path, PublicKeyFile)

	m.logger.WithFields(logrus.Fields{
		"search_path": path,
		"private_key": privateKeyPath,
		"public_key":  publicKeyPath,
	}).Debug("Loading JWT keys from path")

	if entries, err := os.ReadDir(path); err == nil {
		var files []string
		for _, entry := range entries {
			files = append(files, entry.Name())
		}
		m.logger.WithFields(logrus.Fields{
			"directory": path,
			"files":     files,
		}).Debug("Files found in key directory")
	}

	if _, err := os.Stat(privateKeyPath); os.IsNotExist(err) {
		return fmt.Errorf("JWT private key not found at %s\n\n💡 Generate keys first with: p0-ssh-agent keygen --path %s", privateKeyPath, path)
	}

	privateJWK, err := m.loadPrivateJWK(privateKeyPath)
	if err != nil {
		return fmt.Errorf("failed to load private JWK from %s: %w\n\n💡 The key file exists but is invalid. Try regenerating with: p0-ssh-agent keygen --path %s --force", privateKeyPath, err, path)
	}

	publicJWK, err := m.loadPublicJWK(publicKeyPath)
	if err != nil {
		return fmt.Errorf("failed to load public JWK from %s: %w", publicKeyPath, err)
	}

	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.ES384, Key: privateJWK}, (&jose.SignerOptions{}).WithType("JWT"))
	if err != nil {
		return fmt.Errorf("failed to create signer: %w", err)
	}

	m.privateJWK = privateJWK
	m.publicJWK = publicJWK
	m.signer = signer
	m.logger.WithField("path", privateKeyPath).Info("Successfully loaded JWT JWK keys")
	return nil
}

func (m *Manager) GenerateKeyPair(path string) error {
	if err := m.checkDirectoryPermissions(path); err != nil {
		return fmt.Errorf("JWT key directory not accessible: %w", err)
	}

	m.logger.WithField("path", path).Info("Generating new JWT JWK key pair")

	// Generate ECDSA key pair for ES384 (P-384 curve)
	privateKey, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate key pair: %w", err)
	}

	privateJWK := jose.JSONWebKey{
		Key:       privateKey,
		KeyID:     "", // Can be set if needed
		Algorithm: string(jose.ES384),
		Use:       "sig",
	}

	publicJWK := jose.JSONWebKey{
		Key:       &privateKey.PublicKey,
		KeyID:     "", // Should match private key ID if set
		Algorithm: string(jose.ES384),
		Use:       "sig",
	}

	privateKeyPath := filepath.Join(path, PrivateKeyFile)
	if err := m.saveJWK(privateKeyPath, privateJWK, true); err != nil {
		return fmt.Errorf("failed to save private JWK: %w", err)
	}

	publicKeyPath := filepath.Join(path, PublicKeyFile)
	if err := m.saveJWK(publicKeyPath, publicJWK, false); err != nil {
		return fmt.Errorf("failed to save public JWK: %w", err)
	}

	if err := os.Chmod(privateKeyPath, 0400); err != nil {
		m.logger.WithError(err).Warn("Failed to set restrictive permissions on private key")
	}

	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.ES384, Key: privateJWK}, (&jose.SignerOptions{}).WithType("JWT"))
	if err != nil {
		return fmt.Errorf("failed to create signer: %w", err)
	}

	m.privateJWK = privateJWK
	m.publicJWK = publicJWK
	m.signer = signer

	m.logger.Info("Generated new ES384 JWK key pair")
	return nil
}

func (m *Manager) checkDirectoryPermissions(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(path, 0700); err != nil {
				return fmt.Errorf("cannot create directory %s: %w (try: mkdir -p %s && chmod 700 %s)", path, err, path, path)
			}
			m.logger.WithField("path", path).Info("Created JWT key directory")
			return nil
		}
		return fmt.Errorf("cannot access directory %s: %w", path, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", path)
	}

	testFile := filepath.Join(path, ".p0-write-test")
	file, err := os.Create(testFile)
	if err != nil {
		return fmt.Errorf("directory %s is not writable: %w (try: chmod 755 %s)", path, err, path)
	}
	file.Close()
	os.Remove(testFile)

	return nil
}

func (m *Manager) loadPrivateJWK(path string) (jose.JSONWebKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return jose.JSONWebKey{}, fmt.Errorf("cannot read JWK file: %w", err)
	}

	var jwk jose.JSONWebKey
	if err := json.Unmarshal(data, &jwk); err != nil {
		preview := string(data)
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		return jose.JSONWebKey{}, fmt.Errorf("failed to parse JWK JSON: %w\nFile content preview: %s", err, preview)
	}

	return jwk, nil
}

func (m *Manager) loadPublicJWK(path string) (jose.JSONWebKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return jose.JSONWebKey{}, fmt.Errorf("cannot read JWK file: %w", err)
	}

	var jwk jose.JSONWebKey
	if err := json.Unmarshal(data, &jwk); err != nil {
		preview := string(data)
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		return jose.JSONWebKey{}, fmt.Errorf("failed to parse JWK JSON: %w\nFile content preview: %s", err, preview)
	}

	return jwk, nil
}

func (m *Manager) saveJWK(path string, jwk jose.JSONWebKey, includePrivate bool) error {
	var data []byte
	var err error

	if includePrivate {
		data, err = json.MarshalIndent(jwk, "", "  ")
	} else {
		publicJWK := jwk.Public()
		data, err = json.MarshalIndent(publicJWK, "", "  ")
	}

	if err != nil {
		return fmt.Errorf("failed to marshal JWK: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

func (m *Manager) CreateJWT(clientID string) (string, error) {
	if m.signer == nil {
		return "", fmt.Errorf("signer not initialized - call LoadKey or GenerateKeyPair first")
	}

	now := time.Now()
	claims := CustomClaims{
		TunnelID: "my-tunnel-id",
		Claims: jwt.Claims{
			Issuer:   "kd-client",
			Subject:  clientID,
			Audience: jwt.Audience{"p0.dev"},
			IssuedAt: jwt.NewNumericDate(now),
			Expiry:   jwt.NewNumericDate(now.Add(7 * 24 * time.Hour)), // One week
		},
	}

	token, err := jwt.Signed(m.signer).Claims(claims).CompactSerialize()
	if err != nil {
		return "", fmt.Errorf("failed to create JWT: %w", err)
	}

	return token, nil
}
