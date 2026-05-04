// SPDX-License-Identifier: AGPL-3.0-or-later
package check

import "github.com/TRC-Loop/cairn/internal/auth"

// GenerateToken returns 32 random bytes, base64-url-encoded, unpadded.
// Suitable for use as a push check token.
func GenerateToken() (string, error) {
	return auth.GenerateSessionID()
}
