package shadowdeploy

import (
	"sort"

	"github.com/google/uuid"
)

// Divergence kind vocabulary: which side selected a diverging ticker.
const (
	KindShadowOnly = "shadow_only"
	KindActiveOnly = "active_only"
)

// validDivergenceKind reports whether k is one of the two allowed kinds.
func validDivergenceKind(k string) bool {
	return k == KindShadowOnly || k == KindActiveOnly
}

// DivergentTicker is one ticker selected by exactly one side, carrying both
// sides' scores (nil when that side left the ticker unscored, e.g. no_model)
// and the scan result id for drill-down.
type DivergentTicker struct {
	Ticker       string    `json:"ticker"`
	ActiveScore  *float64  `json:"active_score"`
	ShadowScore  *float64  `json:"shadow_score"`
	ScanResultID uuid.UUID `json:"scan_result_id"`
}

// DivergenceReport is the deterministic comparison of two decision vectors
// over the same observations. Selected-set membership (by ticker) is the unit
// of divergence: score differences on commonly-selected tickers never count.
type DivergenceReport struct {
	// TotalObservations is the size of the replayed observation batch.
	TotalObservations int `json:"total_observations"`

	// SelectedActive / SelectedShadow / SelectedBoth count distinct
	// selected tickers per side and in the intersection.
	SelectedActive int `json:"selected_active"`
	SelectedShadow int `json:"selected_shadow"`
	SelectedBoth   int `json:"selected_both"`

	// ShadowOnly and ActiveOnly list the diverging tickers, ascending.
	ShadowOnly []DivergentTicker `json:"shadow_only"`
	ActiveOnly []DivergentTicker `json:"active_only"`

	// DivergencePct = |symmetric difference of the selected sets| /
	// max(1, |union of the selected sets|) x 100. Empty selections on both
	// sides therefore yield 0, not a division by zero.
	DivergencePct float64 `json:"divergence_pct"`
}

// tickerView is one ticker's representative decision data on one side: its
// first decision (for score and scan result id) plus whether any decision
// for the ticker was selected.
type tickerView struct {
	score        *float64
	scanResultID uuid.UUID
	selected     bool
}

// viewByTicker collapses a decision vector to per-ticker views. The first
// occurrence of a ticker is its representative for score and scan result id
// (input order is deterministic, so this is too); the ticker counts as
// selected when any of its decisions is.
func viewByTicker(decisions []Decision) map[string]tickerView {
	views := make(map[string]tickerView, len(decisions))
	for _, d := range decisions {
		v, seen := views[d.Ticker]
		if !seen {
			v = tickerView{score: d.ScorePtr(), scanResultID: d.ScanResultID}
		}
		v.selected = v.selected || d.Selected
		views[d.Ticker] = v
	}
	return views
}

// CompareDecisions computes the divergence report between the active and
// shadow decision vectors, which MUST come from EvaluateConfig runs over the
// same observation slice. Pure and deterministic: diverging tickers are
// ordered ascending.
func CompareDecisions(active, shadow []Decision) DivergenceReport {
	activeViews := viewByTicker(active)
	shadowViews := viewByTicker(shadow)

	report := DivergenceReport{
		TotalObservations: len(active),
		ShadowOnly:        []DivergentTicker{},
		ActiveOnly:        []DivergentTicker{},
	}

	tickers := make([]string, 0, len(activeViews))
	for ticker := range activeViews {
		tickers = append(tickers, ticker)
	}
	for ticker := range shadowViews {
		if _, ok := activeViews[ticker]; !ok {
			tickers = append(tickers, ticker)
		}
	}
	sort.Strings(tickers)

	union := 0
	for _, ticker := range tickers {
		av := activeViews[ticker]
		sv := shadowViews[ticker]

		if av.selected {
			report.SelectedActive++
		}
		if sv.selected {
			report.SelectedShadow++
		}

		switch {
		case av.selected && sv.selected:
			report.SelectedBoth++
			union++
		case sv.selected:
			report.ShadowOnly = append(report.ShadowOnly, DivergentTicker{
				Ticker:       ticker,
				ActiveScore:  av.score,
				ShadowScore:  sv.score,
				ScanResultID: sv.scanResultID,
			})
			union++
		case av.selected:
			report.ActiveOnly = append(report.ActiveOnly, DivergentTicker{
				Ticker:       ticker,
				ActiveScore:  av.score,
				ShadowScore:  sv.score,
				ScanResultID: av.scanResultID,
			})
			union++
		}
	}

	symmetricDifference := len(report.ShadowOnly) + len(report.ActiveOnly)
	divisor := union
	if divisor < 1 {
		divisor = 1
	}
	report.DivergencePct = float64(symmetricDifference) / float64(divisor) * 100

	return report
}
