// SPDX-License-Identifier: AGPL-3.0-or-later
package statuspage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/TRC-Loop/cairn/internal/store"
)

var (
	ErrDomainInvalidFormat = errors.New("invalid domain format")
	ErrDomainConflict      = errors.New("domain already assigned")
	ErrDomainNotFound      = errors.New("domain not found")
)

var domainRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*$`)

func NormalizeDomain(d string) string {
	d = strings.TrimSpace(strings.ToLower(d))
	if i := strings.LastIndex(d, ":"); i != -1 {
		if _, err := fmtAtoi(d[i+1:]); err == nil {
			d = d[:i]
		}
	}
	return d
}

func fmtAtoi(s string) (int, error) {
	n := 0
	if s == "" {
		return 0, errors.New("empty")
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, errors.New("nan")
		}
		n = n*10 + int(r-'0')
	}
	return n, nil
}

func ValidateDomain(d string) error {
	if len(d) == 0 || len(d) > 253 {
		return ErrDomainInvalidFormat
	}
	if !domainRe.MatchString(d) {
		return ErrDomainInvalidFormat
	}
	return nil
}

type DomainCache struct {
	mu sync.RWMutex
	m  map[string]int64
}

func NewDomainCache() *DomainCache {
	return &DomainCache{m: map[string]int64{}}
}

func (c *DomainCache) Get(domain string) (int64, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	id, ok := c.m[domain]
	return id, ok
}

func (c *DomainCache) Reload(ctx context.Context, q *store.Queries) error {
	rows, err := q.ListAllStatusPageDomains(ctx)
	if err != nil {
		return fmt.Errorf("list domains: %w", err)
	}
	next := make(map[string]int64, len(rows))
	for _, r := range rows {
		next[r.Domain] = r.StatusPageID
	}
	c.mu.Lock()
	c.m = next
	c.mu.Unlock()
	return nil
}

func (s *Service) ListDomains(ctx context.Context, statusPageID int64) ([]store.StatusPageDomain, error) {
	return s.q.ListDomainsForStatusPage(ctx, statusPageID)
}

func (s *Service) AddDomain(ctx context.Context, statusPageID int64, domain string) (store.StatusPageDomain, error) {
	domain = NormalizeDomain(domain)
	if err := ValidateDomain(domain); err != nil {
		return store.StatusPageDomain{}, err
	}
	row, err := s.q.AddStatusPageDomain(ctx, store.AddStatusPageDomainParams{
		StatusPageID: statusPageID,
		Domain:       domain,
	})
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") || strings.Contains(strings.ToLower(err.Error()), "constraint") {
			return store.StatusPageDomain{}, ErrDomainConflict
		}
		return store.StatusPageDomain{}, fmt.Errorf("add domain: %w", err)
	}
	return row, nil
}

func (s *Service) RemoveDomain(ctx context.Context, statusPageID, domainID int64) error {
	if _, err := s.q.GetStatusPageDomain(ctx, domainID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrDomainNotFound
		}
		return err
	}
	return s.q.DeleteStatusPageDomain(ctx, store.DeleteStatusPageDomainParams{
		ID:           domainID,
		StatusPageID: statusPageID,
	})
}

func (s *Service) LookupByDomain(ctx context.Context, domain string) (store.StatusPage, error) {
	return s.q.LookupStatusPageByDomain(ctx, NormalizeDomain(domain))
}
