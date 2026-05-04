// SPDX-License-Identifier: AGPL-3.0-or-later
package auth

import (
	"crypto/rand"
	"encoding/base64"
)

// GenerateSessionID returns 32 random bytes, base64-url-encoded, unpadded.
func GenerateSessionID() (string, error) {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf[:]), nil
}
