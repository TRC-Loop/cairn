// SPDX-License-Identifier: AGPL-3.0-or-later
package check

import (
	"regexp"
	"strings"
)

// Matches the "user:password" prefix in a URL-style DSN: scheme://user:password@host
var dsnURLCredsRE = regexp.MustCompile(`(://[^:/@]+):([^@]*)@`)

// Matches "user:password@" in mysql DSN form: user:password@tcp(host:port)/db
var dsnPlainCredsRE = regexp.MustCompile(`^([^:@/\s]+):([^@]*)@`)

// RedactDSN returns a copy of the DSN safe to log.
// The password portion is replaced with ****.
func RedactDSN(dsn string) string {
	if s := dsnURLCredsRE.ReplaceAllString(dsn, "$1:****@"); s != dsn {
		return s
	}
	return dsnPlainCredsRE.ReplaceAllString(dsn, "$1:****@")
}

// redactDBError makes a best-effort attempt to scrub a DSN (and its password)
// out of an error returned by a database driver before it lands in logs.
func redactDBError(err error, dsn string) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if dsn != "" && strings.Contains(msg, dsn) {
		msg = strings.ReplaceAll(msg, dsn, RedactDSN(dsn))
	}
	return msg
}
