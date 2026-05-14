// SPDX-License-Identifier: AGPL-3.0-or-later
package statuspage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/TRC-Loop/cairn/internal/auth"
	"github.com/TRC-Loop/cairn/internal/store"
)

type Service struct {
	db     *sql.DB
	q      *store.Queries
	logger *slog.Logger
}

func NewService(db *sql.DB, q *store.Queries, logger *slog.Logger) *Service {
	return &Service{db: db, q: q, logger: logger}
}

type CreateInput struct {
	Slug             string
	Title            string
	Description      string
	LogoURL          string
	AccentColor      string
	CustomFooterHTML string
	IsDefault        bool
}

type UpdateInput struct {
	Title            string
	Description      string
	LogoURL          string
	AccentColor      string
	CustomFooterHTML string
}

func (s *Service) Get(ctx context.Context, id int64) (store.StatusPage, error) {
	return s.q.GetStatusPage(ctx, id)
}

func (s *Service) GetBySlug(ctx context.Context, slug string) (store.StatusPage, error) {
	return s.q.GetStatusPageBySlug(ctx, slug)
}

func (s *Service) GetDefault(ctx context.Context) (store.StatusPage, error) {
	return s.q.GetDefaultStatusPage(ctx)
}

func (s *Service) List(ctx context.Context) ([]store.StatusPage, error) {
	return s.q.ListStatusPages(ctx)
}

func (s *Service) Create(ctx context.Context, in CreateInput) (store.StatusPage, error) {
	if in.Slug == "" {
		return store.StatusPage{}, errors.New("slug required")
	}
	if in.Title == "" {
		return store.StatusPage{}, errors.New("title required")
	}
	if !in.IsDefault {
		return s.q.CreateStatusPage(ctx, buildCreateParams(in))
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return store.StatusPage{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()
	qtx := s.q.WithTx(tx)
	if err := qtx.UnsetAllDefaults(ctx); err != nil {
		return store.StatusPage{}, fmt.Errorf("unset defaults: %w", err)
	}
	page, err := qtx.CreateStatusPage(ctx, buildCreateParams(in))
	if err != nil {
		return store.StatusPage{}, fmt.Errorf("create: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return store.StatusPage{}, fmt.Errorf("commit: %w", err)
	}
	return page, nil
}

func buildCreateParams(in CreateInput) store.CreateStatusPageParams {
	return store.CreateStatusPageParams{
		Slug:             in.Slug,
		Title:            in.Title,
		Description:      nullString(in.Description),
		LogoUrl:          nullString(in.LogoURL),
		AccentColor:      nullString(in.AccentColor),
		CustomFooterHtml: nullString(in.CustomFooterHTML),
		PasswordHash:     sql.NullString{},
		IsDefault:        in.IsDefault,
	}
}

func (s *Service) Update(ctx context.Context, id int64, in UpdateInput) error {
	if in.Title == "" {
		return errors.New("title required")
	}
	return s.q.UpdateStatusPage(ctx, store.UpdateStatusPageParams{
		Title:            in.Title,
		Description:      nullString(in.Description),
		LogoUrl:          nullString(in.LogoURL),
		AccentColor:      nullString(in.AccentColor),
		CustomFooterHtml: nullString(in.CustomFooterHTML),
		ID:               id,
	})
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.q.DeleteStatusPage(ctx, id)
}

func (s *Service) UpdateFlags(ctx context.Context, id int64, hidePoweredBy, showHistory bool) error {
	return s.q.UpdateStatusPageFlags(ctx, store.UpdateStatusPageFlagsParams{
		HidePoweredBy: hidePoweredBy,
		ShowHistory:   showHistory,
		ID:            id,
	})
}

// SetDefault promotes the given page to is_default=1, clearing any previous default.
func (s *Service) SetDefault(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()
	qtx := s.q.WithTx(tx)
	if err := qtx.UnsetAllDefaults(ctx); err != nil {
		return fmt.Errorf("unset defaults: %w", err)
	}
	if err := qtx.SetStatusPageAsDefault(ctx, id); err != nil {
		return fmt.Errorf("set default: %w", err)
	}
	return tx.Commit()
}

// SetPassword stores an argon2id hash; empty plaintext clears password protection.
func (s *Service) SetPassword(ctx context.Context, id int64, plaintext string) error {
	var hash sql.NullString
	if plaintext != "" {
		h, err := auth.Hash(plaintext)
		if err != nil {
			return fmt.Errorf("hash password: %w", err)
		}
		hash = sql.NullString{String: h, Valid: true}
	}
	return s.q.UpdateStatusPagePassword(ctx, store.UpdateStatusPagePasswordParams{
		PasswordHash: hash,
		ID:           id,
	})
}

// VerifyPassword reports whether plaintext matches the stored hash for slug.
// Pages without a password_hash always return true (unprotected).
func (s *Service) VerifyPassword(ctx context.Context, slug string, plaintext string) (bool, error) {
	page, err := s.q.GetStatusPageBySlug(ctx, slug)
	if err != nil {
		return false, fmt.Errorf("get page: %w", err)
	}
	if !page.PasswordHash.Valid || page.PasswordHash.String == "" {
		return true, nil
	}
	return auth.Verify(plaintext, page.PasswordHash.String)
}

func (s *Service) AddComponent(ctx context.Context, statusPageID, componentID, displayOrder int64) error {
	return s.q.AddComponentToStatusPage(ctx, store.AddComponentToStatusPageParams{
		StatusPageID: statusPageID,
		ComponentID:  componentID,
		DisplayOrder: displayOrder,
	})
}

const (
	ShowMonitorsOff            = "off"
	ShowMonitorsDefaultOpen    = "default_open"
	ShowMonitorsDefaultClosed  = "default_closed"
)

func ValidShowMonitorsMode(m string) bool {
	return m == ShowMonitorsOff || m == ShowMonitorsDefaultOpen || m == ShowMonitorsDefaultClosed
}

func (s *Service) SetComponentShowMonitors(ctx context.Context, statusPageID, componentID int64, mode string) error {
	if !ValidShowMonitorsMode(mode) {
		return errors.New("invalid show_monitors mode")
	}
	return s.q.UpdateStatusPageComponentShowMonitors(ctx, store.UpdateStatusPageComponentShowMonitorsParams{
		ShowMonitorsDefault: mode,
		StatusPageID:        statusPageID,
		ComponentID:         componentID,
	})
}

func (s *Service) ListComponentSettings(ctx context.Context, statusPageID int64) ([]store.ListStatusPageComponentSettingsRow, error) {
	return s.q.ListStatusPageComponentSettings(ctx, statusPageID)
}

func (s *Service) RemoveComponent(ctx context.Context, statusPageID, componentID int64) error {
	return s.q.RemoveComponentFromStatusPage(ctx, store.RemoveComponentFromStatusPageParams{
		StatusPageID: statusPageID,
		ComponentID:  componentID,
	})
}

// ReorderComponents sets display_order for each component in the ordered list
// to its position. Components absent from the list are left untouched.
func (s *Service) ReorderComponents(ctx context.Context, statusPageID int64, orderedComponentIDs []int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()
	qtx := s.q.WithTx(tx)
	for idx, cid := range orderedComponentIDs {
		if err := qtx.UpdateStatusPageComponentOrder(ctx, store.UpdateStatusPageComponentOrderParams{
			DisplayOrder: int64(idx),
			StatusPageID: statusPageID,
			ComponentID:  cid,
		}); err != nil {
			return fmt.Errorf("reorder %d: %w", cid, err)
		}
	}
	return tx.Commit()
}

func (s *Service) ListComponents(ctx context.Context, statusPageID int64) ([]store.Component, error) {
	return s.q.ListComponentsForStatusPage(ctx, statusPageID)
}

func (s *Service) ListDirectMonitors(ctx context.Context, statusPageID int64) ([]store.Check, error) {
	return s.q.ListMonitorsForStatusPage(ctx, statusPageID)
}

func (s *Service) SetDirectMonitors(ctx context.Context, statusPageID int64, monitorIDs []int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()
	qtx := s.q.WithTx(tx)
	if err := qtx.RemoveAllMonitorsFromStatusPage(ctx, statusPageID); err != nil {
		return fmt.Errorf("clear monitors: %w", err)
	}
	for idx, mid := range monitorIDs {
		if err := qtx.AddMonitorToStatusPage(ctx, store.AddMonitorToStatusPageParams{
			StatusPageID: statusPageID,
			CheckID:      mid,
			DisplayOrder: int64(idx),
		}); err != nil {
			return fmt.Errorf("add monitor %d: %w", mid, err)
		}
	}
	return tx.Commit()
}

const (
	FooterModeStructured = "structured"
	FooterModeHTML       = "html"
	FooterModeBoth       = "both"

	FooterElementLink      = "link"
	FooterElementText      = "text"
	FooterElementSeparator = "separator"
)

type FooterElementInput struct {
	ElementType  string
	Label        string
	URL          string
	OpenInNewTab bool
}

// ErrFooterElement is returned for per-element validation failures so the
// API layer can map index+field to a structured error envelope.
type ErrFooterElement struct {
	Index int
	Field string
	Code  string
}

func (e *ErrFooterElement) Error() string {
	return fmt.Sprintf("footer element %d: field %s: %s", e.Index, e.Field, e.Code)
}

func (s *Service) ListFooterElements(ctx context.Context, statusPageID int64) ([]store.StatusPageFooterElement, error) {
	return s.q.ListFooterElements(ctx, statusPageID)
}

func (s *Service) ReplaceFooterElements(ctx context.Context, statusPageID int64, elements []FooterElementInput) ([]store.StatusPageFooterElement, error) {
	for i, el := range elements {
		if err := validateFooterElement(i, el); err != nil {
			return nil, err
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()
	qtx := s.q.WithTx(tx)
	if err := qtx.DeleteFooterElementsForPage(ctx, statusPageID); err != nil {
		return nil, fmt.Errorf("delete elements: %w", err)
	}
	for i, el := range elements {
		if _, err := qtx.CreateFooterElement(ctx, store.CreateFooterElementParams{
			StatusPageID: statusPageID,
			ElementType:  el.ElementType,
			Label:        nullString(el.Label),
			Url:          nullString(el.URL),
			OpenInNewTab: el.OpenInNewTab,
			DisplayOrder: int64(i),
		}); err != nil {
			return nil, fmt.Errorf("create element: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return s.q.ListFooterElements(ctx, statusPageID)
}

func (s *Service) SetFooterMode(ctx context.Context, statusPageID int64, mode string) error {
	if !validFooterMode(mode) {
		return errors.New("invalid footer mode")
	}
	return s.q.UpdateStatusPageFooterMode(ctx, store.UpdateStatusPageFooterModeParams{
		FooterMode: mode,
		ID:         statusPageID,
	})
}

func validFooterMode(m string) bool {
	return m == FooterModeStructured || m == FooterModeHTML || m == FooterModeBoth
}

func validateFooterElement(idx int, el FooterElementInput) error {
	switch el.ElementType {
	case FooterElementLink:
		if el.Label == "" {
			return &ErrFooterElement{Index: idx, Field: "label", Code: "required"}
		}
		if len(el.Label) > 100 {
			return &ErrFooterElement{Index: idx, Field: "label", Code: "too_long"}
		}
		if el.URL == "" {
			return &ErrFooterElement{Index: idx, Field: "url", Code: "required"}
		}
		if len(el.URL) > 500 {
			return &ErrFooterElement{Index: idx, Field: "url", Code: "too_long"}
		}
		if !validFooterURL(el.URL) {
			return &ErrFooterElement{Index: idx, Field: "url", Code: "invalid_format"}
		}
	case FooterElementText:
		if el.Label == "" {
			return &ErrFooterElement{Index: idx, Field: "label", Code: "required"}
		}
		if len(el.Label) > 200 {
			return &ErrFooterElement{Index: idx, Field: "label", Code: "too_long"}
		}
		if el.URL != "" {
			return &ErrFooterElement{Index: idx, Field: "url", Code: "invalid_value"}
		}
	case FooterElementSeparator:
		if el.Label != "" || el.URL != "" {
			return &ErrFooterElement{Index: idx, Field: "label", Code: "invalid_value"}
		}
	default:
		return &ErrFooterElement{Index: idx, Field: "element_type", Code: "invalid_value"}
	}
	return nil
}

func validFooterURL(u string) bool {
	low := strings.ToLower(u)
	return strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") || strings.HasPrefix(low, "mailto:")
}

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
