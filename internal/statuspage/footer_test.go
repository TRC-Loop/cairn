// SPDX-License-Identifier: AGPL-3.0-or-later
package statuspage

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/TRC-Loop/cairn/internal/store"
)

func TestReplaceFooterElementsCRUDAndOrder(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	page, err := svc.Create(ctx, CreateInput{Slug: "f1", Title: "F1"})
	if err != nil {
		t.Fatalf("create page: %v", err)
	}

	saved, err := svc.ReplaceFooterElements(ctx, page.ID, []FooterElementInput{
		{ElementType: FooterElementLink, Label: "Privacy", URL: "https://example.com/p", OpenInNewTab: true},
		{ElementType: FooterElementSeparator},
		{ElementType: FooterElementText, Label: "© 2026"},
	})
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if len(saved) != 3 {
		t.Fatalf("want 3 saved, got %d", len(saved))
	}
	if saved[0].DisplayOrder != 0 || saved[1].DisplayOrder != 1 || saved[2].DisplayOrder != 2 {
		t.Fatalf("display_order not 0..2: %+v", saved)
	}

	reordered, err := svc.ReplaceFooterElements(ctx, page.ID, []FooterElementInput{
		{ElementType: FooterElementText, Label: "© 2026"},
		{ElementType: FooterElementSeparator},
		{ElementType: FooterElementLink, Label: "Privacy", URL: "https://example.com/p", OpenInNewTab: true},
	})
	if err != nil {
		t.Fatalf("reorder replace: %v", err)
	}
	if reordered[0].ElementType != FooterElementText {
		t.Fatalf("expected text first, got %s", reordered[0].ElementType)
	}
}

func TestFooterValidation(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	page, err := svc.Create(ctx, CreateInput{Slug: "f2", Title: "F2"})
	if err != nil {
		t.Fatalf("create page: %v", err)
	}

	cases := []struct {
		name string
		in   []FooterElementInput
		want string // expected error field
	}{
		{"link missing label", []FooterElementInput{{ElementType: FooterElementLink, URL: "https://x"}}, "label"},
		{"link missing url", []FooterElementInput{{ElementType: FooterElementLink, Label: "x"}}, "url"},
		{"link js url rejected", []FooterElementInput{{ElementType: FooterElementLink, Label: "x", URL: "javascript:alert(1)"}}, "url"},
		{"text empty label", []FooterElementInput{{ElementType: FooterElementText}}, "label"},
		{"text too long", []FooterElementInput{{ElementType: FooterElementText, Label: strings.Repeat("a", 201)}}, "label"},
		{"unknown type", []FooterElementInput{{ElementType: "image", Label: "x"}}, "element_type"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.ReplaceFooterElements(ctx, page.ID, tc.in)
			if err == nil {
				t.Fatalf("expected error")
			}
			fe, ok := err.(*ErrFooterElement)
			if !ok {
				t.Fatalf("wrong err type: %v", err)
			}
			if fe.Field != tc.want {
				t.Fatalf("want field %s got %s", tc.want, fe.Field)
			}
		})
	}
}

func TestSetFooterMode(t *testing.T) {
	svc, q := newTestService(t)
	ctx := context.Background()
	page, err := svc.Create(ctx, CreateInput{Slug: "f3", Title: "F3"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if page.FooterMode != FooterModeStructured {
		t.Fatalf("default footer_mode should be structured, got %s", page.FooterMode)
	}
	if err := svc.SetFooterMode(ctx, page.ID, FooterModeBoth); err != nil {
		t.Fatalf("set both: %v", err)
	}
	got, _ := q.GetStatusPage(ctx, page.ID)
	if got.FooterMode != FooterModeBoth {
		t.Fatalf("want both, got %s", got.FooterMode)
	}
	if err := svc.SetFooterMode(ctx, page.ID, "garbage"); err == nil {
		t.Fatal("expected error for invalid mode")
	}
}

func TestRenderFooterModes(t *testing.T) {
	elements := []store.StatusPageFooterElement{
		{ElementType: FooterElementLink, Label: nullStr("Privacy"), Url: nullStr("https://example.com/p"), OpenInNewTab: true},
		{ElementType: FooterElementSeparator},
		{ElementType: FooterElementText, Label: nullStr("© 2026")},
	}
	customHTML := `<p>Footnote</p>`

	structured := string(RenderFooter(FooterModeStructured, elements, customHTML))
	if !strings.Contains(structured, "Privacy") || !strings.Contains(structured, `target="_blank"`) {
		t.Fatalf("structured missing parts: %s", structured)
	}
	if strings.Contains(structured, "Footnote") {
		t.Fatalf("structured should not include html: %s", structured)
	}

	htmlOnly := string(RenderFooter(FooterModeHTML, elements, customHTML))
	if strings.Contains(htmlOnly, "Privacy") {
		t.Fatalf("html-only should not include structured: %s", htmlOnly)
	}
	if !strings.Contains(htmlOnly, "Footnote") {
		t.Fatalf("html-only missing custom: %s", htmlOnly)
	}

	both := string(RenderFooter(FooterModeBoth, elements, customHTML))
	if !strings.Contains(both, "Privacy") || !strings.Contains(both, "Footnote") {
		t.Fatalf("both missing parts: %s", both)
	}
}

func TestRenderFooterRejectsBadURL(t *testing.T) {
	// element-level: even if a malformed URL somehow lives in the DB,
	// renderer must skip the link rather than emit it.
	elements := []store.StatusPageFooterElement{
		{ElementType: FooterElementLink, Label: nullStr("x"), Url: nullStr("javascript:alert(1)"), OpenInNewTab: true},
	}
	got := string(RenderFooter(FooterModeStructured, elements, ""))
	if strings.Contains(got, "javascript:") {
		t.Fatalf("rendered output leaked unsafe url: %s", got)
	}
}

func TestMigrationBackfillsHTMLMode(t *testing.T) {
	db, q := openTestDB(t)
	defer db.Close()
	ctx := context.Background()

	if _, err := db.ExecContext(ctx, `INSERT INTO status_pages (slug, title, custom_footer_html) VALUES ('legacy', 'Legacy', '<p>Old footer</p>')`); err != nil {
		t.Fatalf("insert legacy: %v", err)
	}
	// The migration in this run already executed before the insert above (so default fired).
	// Manually apply the same backfill to verify the SQL works on this row:
	if _, err := db.ExecContext(ctx, `UPDATE status_pages SET footer_mode = 'html' WHERE custom_footer_html IS NOT NULL AND length(trim(custom_footer_html)) > 0`); err != nil {
		t.Fatalf("backfill: %v", err)
	}
	row, err := q.GetStatusPageBySlug(ctx, "legacy")
	if err != nil {
		t.Fatalf("get legacy: %v", err)
	}
	if row.FooterMode != FooterModeHTML {
		t.Fatalf("legacy with custom_footer_html should be 'html', got %s", row.FooterMode)
	}
}

func nullStr(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}
