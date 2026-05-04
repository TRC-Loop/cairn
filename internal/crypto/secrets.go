// SPDX-License-Identifier: AGPL-3.0-or-later
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

const nonceSize = 12

// SecretBox encrypts/decrypts arbitrary byte slices using AES-256-GCM with a
// key derived from a master key string via SHA-256.
type SecretBox struct {
	aead cipher.AEAD
}

func NewSecretBox(masterKey string) (*SecretBox, error) {
	if len(masterKey) < 32 {
		return nil, errors.New("master key must be at least 32 bytes")
	}
	sum := sha256.Sum256([]byte(masterKey))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, fmt.Errorf("aes cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	return &SecretBox{aead: aead}, nil
}

// Encrypt returns base64-encoded (nonce || ciphertext).
func (s *SecretBox) Encrypt(plaintext []byte) (string, error) {
	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("nonce: %w", err)
	}
	ct := s.aead.Seal(nil, nonce, plaintext, nil)
	out := make([]byte, 0, nonceSize+len(ct))
	out = append(out, nonce...)
	out = append(out, ct...)
	return base64.StdEncoding.EncodeToString(out), nil
}

func (s *SecretBox) EncryptString(plaintext string) (string, error) {
	return s.Encrypt([]byte(plaintext))
}

func (s *SecretBox) Decrypt(b64 string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("base64: %w", err)
	}
	if len(raw) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}
	nonce, ct := raw[:nonceSize], raw[nonceSize:]
	pt, err := s.aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	return pt, nil
}

func (s *SecretBox) DecryptString(b64 string) (string, error) {
	pt, err := s.Decrypt(b64)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}
