// SPDX-License-Identifier: AGPL-3.0-or-later
package incident

import (
	"fmt"
	"strings"
	"time"
)

const DefaultIDFormat = "#INC-{id}"

// FormatID renders an incident's display ID using the format template.
// Supported placeholders:
//   {id}       - incident database ID
//   {year}     - 4-digit year of started_at (UTC)
//   {month}    - 2-digit month
//   {day}      - 2-digit day
//   {datetime} - 'YYYYMMDDTHHMMSS' of started_at (UTC, no separators)
// Unknown placeholders are passed through unchanged.
func FormatID(template string, incidentID int64, startedAt time.Time) string {
	if template == "" {
		template = DefaultIDFormat
	}
	t := startedAt.UTC()
	out := template
	out = strings.ReplaceAll(out, "{id}", fmt.Sprintf("%d", incidentID))
	out = strings.ReplaceAll(out, "{year}", t.Format("2006"))
	out = strings.ReplaceAll(out, "{month}", t.Format("01"))
	out = strings.ReplaceAll(out, "{day}", t.Format("02"))
	out = strings.ReplaceAll(out, "{datetime}", t.Format("20060102T150405"))
	return out
}
