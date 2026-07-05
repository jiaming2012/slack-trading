package evtracker

import (
	"sort"
	"time"
)

// BucketEVs partitions trades into consecutive equal-length time buckets of
// bucketDays width, anchored at the earliest trade, and returns the per-bucket
// EV for every bucket that contains at least one decided (non-breakeven) trade,
// ordered oldest-to-newest. Buckets with no decided trades are omitted so they
// do not distort the slope. The caller is expected to pass trades already
// filtered to the as-of horizon.
func BucketEVs(trades []TradeOutcome, bucketDays int) []float64 {
	if len(trades) == 0 {
		return nil
	}
	if bucketDays <= 0 {
		bucketDays = DefaultBucketDays
	}
	bucketDur := time.Duration(bucketDays) * hoursPerDay

	minTime := trades[0].ClosedAt
	for _, t := range trades[1:] {
		if t.ClosedAt.Before(minTime) {
			minTime = t.ClosedAt
		}
	}

	buckets := make(map[int][]TradeOutcome)
	for _, t := range trades {
		idx := int(t.ClosedAt.Sub(minTime) / bucketDur)
		buckets[idx] = append(buckets[idx], t)
	}

	indices := make([]int, 0, len(buckets))
	for idx := range buckets {
		indices = append(indices, idx)
	}
	sort.Ints(indices)

	evs := make([]float64, 0, len(indices))
	for _, idx := range indices {
		stats := ComputeEV(buckets[idx])
		if stats.Decided == 0 {
			continue
		}
		evs = append(evs, stats.EV)
	}
	return evs
}

// OLSSlope fits an ordinary-least-squares slope of the supplied per-bucket EV
// values against their integer index 0, 1, ..., n-1. It returns ok == false
// when fewer than two points exist (an undefined slope), in which case the
// returned slope is zero and must be treated as null by the caller.
func OLSSlope(points []float64) (float64, bool) {
	n := len(points)
	if n < 2 {
		return 0, false
	}

	nf := float64(n)
	var sumX, sumY, sumXY, sumXX float64
	for i, y := range points {
		x := float64(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumXX += x * x
	}

	denom := nf*sumXX - sumX*sumX
	if denom == 0 {
		return 0, false
	}
	slope := (nf*sumXY - sumX*sumY) / denom
	return slope, true
}
