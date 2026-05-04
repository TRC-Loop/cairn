// SPDX-License-Identifier: AGPL-3.0-or-later
package statuspage

import (
	"html/template"

	"github.com/microcosm-cc/bluemonday"
)

var footerPolicy = buildFooterPolicy()

func buildFooterPolicy() *bluemonday.Policy {
	p := bluemonday.NewPolicy()
	p.AllowElements("a", "p", "br", "strong", "em", "span")
	p.AllowAttrs("href", "target", "rel").OnElements("a")
	p.AllowURLSchemes("http", "https", "mailto")
	p.RequireParseableURLs(true)
	p.AddTargetBlankToFullyQualifiedLinks(true)
	return p
}

// SanitizeFooter scrubs admin-authored footer HTML. Output is safe for direct
// rendering: scripts, event handlers, and javascript: URLs are stripped.
func SanitizeFooter(raw string) template.HTML {
	return template.HTML(footerPolicy.Sanitize(raw))
}
