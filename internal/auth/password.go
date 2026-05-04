// SPDX-License-Identifier: AGPL-3.0-or-later
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonMemory      uint32 = 64 * 1024
	argonIterations  uint32 = 3
	argonParallelism uint8  = 4
	argonSaltLen     uint32 = 16
	argonKeyLen      uint32 = 32
)

var (
	ErrInvalidHash     = errors.New("invalid password hash format")
	ErrUnsupportedAlgo = errors.New("unsupported password hash algorithm")
)

// Hash returns an argon2id PHC-format string for plaintext.
func Hash(plaintext string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	key := argon2.IDKey([]byte(plaintext), salt, argonIterations, argonMemory, argonParallelism, argonKeyLen)
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		argonMemory,
		argonIterations,
		argonParallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// Verify checks whether plaintext matches a PHC-format argon2id hash.
func Verify(plaintext string, storedHash string) (bool, error) {
	params, salt, key, err := decodeHash(storedHash)
	if err != nil {
		return false, err
	}
	computed := argon2.IDKey([]byte(plaintext), salt, params.iterations, params.memory, params.parallelism, uint32(len(key)))
	if subtle.ConstantTimeCompare(key, computed) == 1 {
		return true, nil
	}
	return false, nil
}

type argonParams struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
}

func decodeHash(encoded string) (argonParams, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" {
		return argonParams{}, nil, nil, ErrInvalidHash
	}
	if parts[1] != "argon2id" {
		return argonParams{}, nil, nil, ErrUnsupportedAlgo
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return argonParams{}, nil, nil, fmt.Errorf("parse version: %w", err)
	}
	if version != argon2.Version {
		return argonParams{}, nil, nil, ErrUnsupportedAlgo
	}
	var p argonParams
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.memory, &p.iterations, &p.parallelism); err != nil {
		return argonParams{}, nil, nil, fmt.Errorf("parse params: %w", err)
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return argonParams{}, nil, nil, fmt.Errorf("decode salt: %w", err)
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return argonParams{}, nil, nil, fmt.Errorf("decode key: %w", err)
	}
	return p, salt, key, nil
}
