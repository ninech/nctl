package test

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/pem"
	"fmt"
	"strings"

	"golang.org/x/crypto/ed25519"
	"golang.org/x/crypto/ssh"
)

const (
	keySize = 4096
)

// GenerateRSAPrivateKey generates an RSA private key
func GenerateRSAPrivateKey() (string, error) {
	// Private Key generation
	privateKey, err := rsa.GenerateKey(rand.Reader, keySize)
	if err != nil {
		return "", err
	}
	// Validate Private Key
	err = privateKey.Validate()
	if err != nil {
		return "", err
	}
	privBlock, err := ssh.MarshalPrivateKey(privateKey, "")
	if err != nil {
		return "", err
	}
	// Private key in PEM format
	return strings.TrimSpace(string(pem.EncodeToMemory(privBlock))), nil
}

// GenerateED25519PrivateKey generates an ED25519 private key
func GenerateED25519PrivateKey() (string, error) {
	_, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		return "", fmt.Errorf("can not create ed25519 key: %w", err)
	}
	pemKey, err := ssh.MarshalPrivateKey(privateKey, "")
	if err != nil {
		return "", fmt.Errorf("can not marshal ed25519 private key: %w", err)
	}

	// Private key in PEM format
	return strings.TrimSpace(string(pem.EncodeToMemory(pemKey))), nil
}
