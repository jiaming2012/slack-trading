package riskoverlay

import (
	"fmt"
	"sync"
	"time"

	"gorm.io/gorm"
)

// SectorLookup resolves a ticker's sector for the sector-concentration limit
// family. An UNKNOWN ticker resolves to the empty sector with no error — the
// engine skips the sector-concentration check for an empty sector, and the
// snapshot builder records the proposed entry's unknown-sector resolution as
// degradation (reason sector_unknown) so partial blindness of the family stays
// operator-visible. A lookup ERROR (DB failure) surfaces as a snapshot-build
// error, handled fail-permissive by the gate.
type SectorLookup interface {
	SectorOf(ticker string) (string, error)
}

// GormSectorLookup is the production SectorLookup: it reads the latest
// non-null scan_results.sector for a ticker (scan_results is the platform's
// sector source of truth) and never writes.
type GormSectorLookup struct {
	db *gorm.DB
}

// NewGormSectorLookup constructs a GormSectorLookup over an already migrated
// *gorm.DB (see tradingstack.MigrateTradingStack).
func NewGormSectorLookup(db *gorm.DB) *GormSectorLookup {
	return &GormSectorLookup{db: db}
}

// SectorOf returns the ticker's latest known sector, or "" when no scan result
// carries a sector for it.
func (g *GormSectorLookup) SectorOf(ticker string) (string, error) {
	var sectors []string
	err := g.db.
		Raw(`SELECT sector
		     FROM scan_results
		     WHERE ticker = ? AND sector IS NOT NULL
		     ORDER BY scanned_at DESC
		     LIMIT 1`, ticker).
		Scan(&sectors).Error
	if err != nil {
		return "", fmt.Errorf("GormSectorLookup.SectorOf(%q): load scan_results sector failed: %w", ticker, err)
	}
	if len(sectors) == 0 {
		return "", nil
	}
	return sectors[0], nil
}

// FakeSectorLookup is an in-memory, seedable SectorLookup for unit tests. Set
// Err to simulate a lookup outage; Calls counts SectorOf invocations so tests
// can assert the reduction-bypass and cache behavior.
type FakeSectorLookup struct {
	Sectors map[string]string
	Err     error
	Calls   int
}

// NewFakeSectorLookup constructs a fake seeded with ticker -> sector.
func NewFakeSectorLookup(sectors map[string]string) *FakeSectorLookup {
	return &FakeSectorLookup{Sectors: sectors}
}

// SectorOf returns the seeded sector, or "" for an unseeded ticker.
func (f *FakeSectorLookup) SectorOf(ticker string) (string, error) {
	f.Calls++
	if f.Err != nil {
		return "", f.Err
	}
	return f.Sectors[ticker], nil
}

// CachedSectorLookup memoizes per-ticker sector resolutions (including the
// negative "no sector" result — an option OCC symbol will never gain one) for
// a TTL, keeping the per-order evaluation path flat. Errors are never cached.
// Safe for concurrent use.
type CachedSectorLookup struct {
	inner SectorLookup
	ttl   time.Duration
	now   func() time.Time

	mu      sync.Mutex
	entries map[string]sectorCacheEntry
}

type sectorCacheEntry struct {
	sector string
	at     time.Time
}

// NewCachedSectorLookup wraps inner with a per-ticker TTL cache. A
// non-positive ttl falls back to DefaultLookupCacheTTL.
func NewCachedSectorLookup(inner SectorLookup, ttl time.Duration) *CachedSectorLookup {
	if ttl <= 0 {
		ttl = DefaultLookupCacheTTL
	}
	return &CachedSectorLookup{inner: inner, ttl: ttl, now: time.Now, entries: make(map[string]sectorCacheEntry)}
}

// SectorOf returns the cached sector when fresh, otherwise consults the inner
// lookup.
func (c *CachedSectorLookup) SectorOf(ticker string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if e, ok := c.entries[ticker]; ok && c.now().Sub(e.at) < c.ttl {
		return e.sector, nil
	}

	sector, err := c.inner.SectorOf(ticker)
	if err != nil {
		return "", err
	}
	c.entries[ticker] = sectorCacheEntry{sector: sector, at: c.now()}
	return sector, nil
}
