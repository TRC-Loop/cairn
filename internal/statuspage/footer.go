// SPDX-License-Identifier: AGPL-3.0-or-later
package statuspage

import (
	"html"
	"html/template"
	"strings"

	"github.com/TRC-Loop/cairn/internal/store"
)

// RenderFooter builds the page footer markup according to mode. Output is
// already sanitized (structured side via controlled construction; html side
// via SanitizeFooter). Returns empty string when there is nothing to render.
func RenderFooter(mode string, elements []store.StatusPageFooterElement, customHTML string) template.HTML {
	var b strings.Builder
	switch mode {
	case FooterModeStructured:
		writeStructured(&b, elements)
	case FooterModeHTML:
		writeRawHTML(&b, customHTML)
	case FooterModeBoth:
		writeStructured(&b, elements)
		writeRawHTML(&b, customHTML)
	default:
		writeStructured(&b, elements)
	}
	return template.HTML(b.String())
}

func writeStructured(b *strings.Builder, elements []store.StatusPageFooterElement) {
	if len(elements) == 0 {
		return
	}
	b.WriteString(`<div class="status-page-footer">`)
	for _, el := range elements {
		switch el.ElementType {
		case FooterElementLink:
			renderLink(b, el)
		case FooterElementText:
			b.WriteString(`<span class="footer-text">`)
			b.WriteString(html.EscapeString(el.Label.String))
			b.WriteString(`</span>`)
		case FooterElementSeparator:
			b.WriteString(`<span class="footer-sep" aria-hidden="true">·</span>`)
		}
	}
	b.WriteString(`</div>`)
}

func renderLink(b *strings.Builder, el store.StatusPageFooterElement) {
	url := el.Url.String
	if !validFooterURL(url) {
		return
	}
	b.WriteString(`<a class="footer-link" href="`)
	b.WriteString(html.EscapeString(url))
	b.WriteString(`"`)
	if el.OpenInNewTab {
		b.WriteString(` target="_blank" rel="noopener noreferrer"`)
	}
	b.WriteString(`>`)
	b.WriteString(html.EscapeString(el.Label.String))
	b.WriteString(`</a>`)
}

func writeRawHTML(b *strings.Builder, customHTML string) {
	if strings.TrimSpace(customHTML) == "" {
		return
	}
	b.WriteString(`<div class="status-page-footer footer-html">`)
	b.WriteString(string(SanitizeFooter(customHTML)))
	b.WriteString(`</div>`)
}
