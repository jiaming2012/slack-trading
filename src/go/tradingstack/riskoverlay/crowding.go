package riskoverlay

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/jiaming2012/slack-trading/src/go/tradingstack/crowding"
)

// CrowdingView is the current scan cycle's crowding result as consumed by the
// engine: whether the cycle was flagged and the set of tickers named in
// crowding_flagged_candidates. It is an immutable value; construct it with
// NewCrowdingView.
type CrowdingView struct {
	Flagged bool
	tickers map[string]bool
}

// NewCrowdingView builds a CrowdingView from a flagged bool and the flagged
// ticker list. A nil or empty ticker list yields a view whose IsFlagged is
// always false.
func NewCrowdingView(flagged bool, tickers []string) CrowdingView {
	set := make(map[string]bool, len(tickers))
	for _, t := range tickers {
		set[t] = true
	}
	return CrowdingView{Flagged: flagged, tickers: set}
}

// IsFlagged reports whether the given ticker is in this cycle's flagged-crowded
// set.
func (v CrowdingView) IsFlagged(ticker string) bool {
	return v.tickers[ticker]
}

// Tickers returns a copy of the flagged ticker set as a slice (order
// unspecified). Provided for test assertions and diagnostics.
func (v CrowdingView) Tickers() []string {
	out := make([]string, 0, len(v.tickers))
	for t := range v.tickers {
		out = append(out, t)
	}
	return out
}

// CrowdingLookup resolves the CrowdingView for a scan cycle. The production
// implementation reads the crowding tables; the test fake is in-memory.
type CrowdingLookup interface {
	// ViewForScanCycle returns the crowding view for the cycle scanned at the
	// given time. When no crowding metric exists for that cycle, it returns an
	// unflagged view with an empty ticker set and no error.
	ViewForScanCycle(scannedAt time.Time) (CrowdingView, error)
}

// GormCrowdingLookup is the production CrowdingLookup, reading crowding_metrics
// and crowding_flagged_candidates for a scanned_at. It never writes.
type GormCrowdingLookup struct {
	db *gorm.DB
}

// NewGormCrowdingLookup constructs a GormCrowdingLookup over an already
// migrated *gorm.DB (see crowding.MigrateCrowdingDetection).
func NewGormCrowdingLookup(db *gorm.DB) *GormCrowdingLookup {
	return &GormCrowdingLookup{db: db}
}

// ViewForScanCycle loads the most recent crowding metric for the given
// scanned_at and, when that metric is flagged, the tickers of its flagged
// candidates.
func (g *GormCrowdingLookup) ViewForScanCycle(scannedAt time.Time) (CrowdingView, error) {
	var metric crowding.CrowdingMetric
	err := g.db.
		Where("scanned_at = ?", scannedAt).
		Order("computed_at DESC").
		First(&metric).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return NewCrowdingView(false, nil), nil
		}
		return CrowdingView{}, fmt.Errorf("GormCrowdingLookup.ViewForScanCycle: load metric failed: %w", err)
	}

	if !metric.Flagged {
		return NewCrowdingView(false, nil), nil
	}

	var candidates []crowding.CrowdingFlaggedCandidate
	if err := g.db.Where("crowding_metric_id = ?", metric.ID).Find(&candidates).Error; err != nil {
		return CrowdingView{}, fmt.Errorf("GormCrowdingLookup.ViewForScanCycle: load flagged candidates failed: %w", err)
	}

	tickers := make([]string, 0, len(candidates))
	for _, c := range candidates {
		tickers = append(tickers, c.Ticker)
	}
	return NewCrowdingView(true, tickers), nil
}

// FakeCrowdingLookup is an in-memory CrowdingLookup for unit tests. Seed it by
// scanned_at; an unseeded scanned_at returns an unflagged, empty view.
type FakeCrowdingLookup struct {
	views map[time.Time]CrowdingView
}

// NewFakeCrowdingLookup constructs an empty FakeCrowdingLookup.
func NewFakeCrowdingLookup() *FakeCrowdingLookup {
	return &FakeCrowdingLookup{views: make(map[time.Time]CrowdingView)}
}

// Seed registers the flagged ticker set for a scan cycle.
func (f *FakeCrowdingLookup) Seed(scannedAt time.Time, flagged bool, tickers []string) {
	f.views[scannedAt] = NewCrowdingView(flagged, tickers)
}

// ViewForScanCycle returns the seeded view for scannedAt, or an unflagged empty
// view when nothing was seeded.
func (f *FakeCrowdingLookup) ViewForScanCycle(scannedAt time.Time) (CrowdingView, error) {
	if v, ok := f.views[scannedAt]; ok {
		return v, nil
	}
	return NewCrowdingView(false, nil), nil
}
