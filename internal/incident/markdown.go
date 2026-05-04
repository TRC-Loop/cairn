// SPDX-License-Identifier: AGPL-3.0-or-later
package incident

import (
	"bytes"
	"html/template"
	"regexp"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

var (
	mdParser   = goldmark.New(goldmark.WithExtensions(extension.Linkify, extension.Strikethrough))
	mdSanitize = mdPolicy()
	stripRe    = regexp.MustCompile(`(?s)\s+`)
)

func mdPolicy() *bluemonday.Policy {
	p := bluemonday.NewPolicy()
	p.AllowElements("p", "br", "strong", "em", "del", "code", "pre", "ul", "ol", "li", "h3", "h4", "blockquote", "span")
	p.AllowAttrs("href").OnElements("a")
	p.RequireParseableURLs(true)
	p.AllowURLSchemes("http", "https", "mailto")
	p.RequireNoFollowOnLinks(true)
	p.AddTargetBlankToFullyQualifiedLinks(true)
	p.AllowElements("a")
	return p
}

// RenderMarkdown converts markdown to sanitized HTML.
func RenderMarkdown(input string) template.HTML {
	if input == "" {
		return ""
	}
	var buf bytes.Buffer
	if err := mdParser.Convert([]byte(input), &buf); err != nil {
		return template.HTML(template.HTMLEscapeString(input))
	}
	clean := mdSanitize.SanitizeBytes(buf.Bytes())
	return template.HTML(clean)
}

// StripMarkdown returns a plain-text approximation of the markdown input.
func StripMarkdown(input string) string {
	if input == "" {
		return ""
	}
	var buf bytes.Buffer
	if err := mdParser.Convert([]byte(input), &buf); err != nil {
		return input
	}
	stripPolicy := bluemonday.StrictPolicy()
	plain := string(stripPolicy.SanitizeBytes(buf.Bytes()))
	plain = strings.TrimSpace(plain)
	plain = stripRe.ReplaceAllString(plain, " ")
	return plain
}
