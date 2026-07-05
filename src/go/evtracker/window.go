package evtracker

import "time"

// hoursPerDay expresses a day as a duration for window arithmetic. Windows are
// defined in whole days relative to a supplied as-of timestamp.
const hoursPerDay = 24 * time.Hour

// Within returns the trades that closed within the inclusive window
// [asOf - days, asOf]. The as-of timestamp is always supplied by the caller and
// never read from the wall clock, so results are reproducible. Trades that
// closed after asOf are excluded from every window.
func Within(trades []TradeOutcome, asOf time.Time, days int) []TradeOutcome {
	cutoff := asOf.Add(-time.Duration(days) * hoursPerDay)
	out := make([]TradeOutcome, 0, len(trades))
	for _, t := range trades {
		if t.ClosedAt.Before(cutoff) || t.ClosedAt.After(asOf) {
			continue
		}
		out = append(out, t)
	}
	return out
}

// AllTime returns every trade that closed at or before asOf, in input order.
// Trades closing strictly after asOf are excluded.
func AllTime(trades []TradeOutcome, asOf time.Time) []TradeOutcome {
	out := make([]TradeOutcome, 0, len(trades))
	for _, t := range trades {
		if t.ClosedAt.After(asOf) {
			continue
		}
		out = append(out, t)
	}
	return out
}
