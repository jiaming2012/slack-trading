package scanner

import (
	"fmt"
	"math"
	"time"
)

const (
	rsiPeriod         = 14
	atrPeriod         = 14
	priceVs50MAPeriod = 50

	// compressionBBPeriod is the Bollinger Band lookback used by
	// ComputeCompressionScore. Unlike rsi_14 / atr_pct / price_vs_50ma, the
	// architecture doc does not mandate a specific period for
	// compression_score -- it only specifies the behavior (percentile rank of
	// current band width against a trailing historical distribution). A
	// smaller period keeps fixture-driven, hand-computed tests tractable.
	compressionBBPeriod = 5

	// nyMarketOpenHour/Minute and nyMarketCloseHour anchor the standard NYSE
	// session (9:30-16:00 America/New_York) used to pace-adjust volume_ratio.
	nyMarketOpenHour    = 9
	nyMarketOpenMinute  = 30
	nyMarketCloseHour   = 16
	nyMarketCloseMinute = 0
)

// closesFromCandles extracts the Close of every candle, in the same
// (ascending) order.
func closesFromCandles(candles []Candle) []float64 {
	closes := make([]float64, len(candles))
	for i, c := range candles {
		closes[i] = c.Close
	}
	return closes
}

// ComputeRSI14 computes the standard 14-period Relative Strength Index over
// the given close-price history (ascending order). It returns nil when fewer
// than 15 closes (14 price changes) are available, rather than fabricating a
// value.
func ComputeRSI14(closes []float64) *float64 {
	if len(closes) < rsiPeriod+1 {
		return nil
	}

	window := closes[len(closes)-(rsiPeriod+1):]

	var gainSum, lossSum float64
	for i := 1; i < len(window); i++ {
		diff := window[i] - window[i-1]
		if diff > 0 {
			gainSum += diff
		} else {
			lossSum += -diff
		}
	}

	avgGain := gainSum / rsiPeriod
	avgLoss := lossSum / rsiPeriod

	var rsi float64
	switch {
	case avgLoss == 0 && avgGain == 0:
		rsi = 50
	case avgLoss == 0:
		rsi = 100
	default:
		rs := avgGain / avgLoss
		rsi = 100 - (100 / (1 + rs))
	}

	return &rsi
}

// ComputeATRPct computes the 14-period Average True Range divided by the
// current price. It returns nil when fewer than 15 candles (14 true ranges,
// each of which needs a prior close) are available, or when price is
// non-positive.
func ComputeATRPct(candles []Candle, price float64) *float64 {
	if len(candles) < atrPeriod+1 || price <= 0 {
		return nil
	}

	window := candles[len(candles)-(atrPeriod+1):]

	var trSum float64
	for i := 1; i < len(window); i++ {
		high, low := window[i].High, window[i].Low
		prevClose := window[i-1].Close

		tr := high - low
		if v := math.Abs(high - prevClose); v > tr {
			tr = v
		}
		if v := math.Abs(low - prevClose); v > tr {
			tr = v
		}
		trSum += tr
	}

	atr := trSum / atrPeriod
	atrPct := atr / price
	return &atrPct
}

// ComputePriceVs50MA computes the signed percentage the current price is
// above or below the 50-day simple moving average of close prices. It
// returns nil when fewer than 50 closes are available.
func ComputePriceVs50MA(closes []float64, price float64) *float64 {
	if len(closes) < priceVs50MAPeriod {
		return nil
	}

	window := closes[len(closes)-priceVs50MAPeriod:]
	var sum float64
	for _, c := range window {
		sum += c
	}
	ma := sum / priceVs50MAPeriod
	if ma == 0 {
		return nil
	}

	pct := (price - ma) / ma * 100
	return &pct
}

// ComputeCompressionScore computes the percentile rank (0-100) of the current
// Bollinger Band width against a trailing historical distribution of
// Bollinger Band widths for the same ticker, using a rolling
// compressionBBPeriod-close window. A narrower-than-typical current width
// ranks lower than a wider-than-typical one. It returns nil when there is no
// historical distribution to rank against (fewer than compressionBBPeriod+1
// closes).
func ComputeCompressionScore(closes []float64) *float64 {
	n := len(closes)
	if n < compressionBBPeriod+1 {
		return nil
	}

	numWindows := n - compressionBBPeriod + 1
	widths := make([]float64, numWindows)
	for i := 0; i < numWindows; i++ {
		widths[i] = bollingerWidth(closes[i : i+compressionBBPeriod])
	}

	current := widths[len(widths)-1]
	historical := widths[:len(widths)-1]
	if len(historical) == 0 {
		return nil
	}

	var countLE int
	for _, w := range historical {
		if w <= current {
			countLE++
		}
	}

	pct := 100 * float64(countLE) / float64(len(historical))
	return &pct
}

// bollingerWidth computes the (upper - lower) / mean Bollinger Band width for
// a window of closes, using a 2-standard-deviation band and population
// standard deviation (divide by N, not N-1).
func bollingerWidth(window []float64) float64 {
	n := float64(len(window))

	var sum float64
	for _, v := range window {
		sum += v
	}
	mean := sum / n
	if mean == 0 {
		return 0
	}

	var sqSum float64
	for _, v := range window {
		d := v - mean
		sqSum += d * d
	}
	variance := sqSum / n
	std := math.Sqrt(variance)

	// upper = mean + 2*std, lower = mean - 2*std => width = (upper-lower)/mean
	return 4 * std / mean
}

// ComputeVolumeRatio computes a pace-adjusted intraday volume ratio: today's
// volume-so-far (the Volume of the last, possibly-partial, candle) normalized
// against the 20-day average volume scaled to the same elapsed-session-time
// fraction, so a partial trading day is compared against a like-for-like
// partial baseline rather than the full-day 20-day average.
//
// Because ScanInput does not carry a full 20-day intraday volume profile, the
// pace-adjusted baseline assumes a uniform intraday volume distribution:
// baseline = elapsed_session_fraction * AvgDailyVolume20d. Modeling a
// non-uniform historical intraday volume curve is left to a future change
// that assembles ScanInput from a live feed with that data available.
//
// It returns nil when there are no candles, AvgDailyVolume20d is
// non-positive, or the elapsed session fraction cannot be determined.
func ComputeVolumeRatio(input ScanInput) *float64 {
	if len(input.Candles) == 0 || input.AvgDailyVolume20d <= 0 {
		return nil
	}

	today := input.Candles[len(input.Candles)-1]

	frac, err := elapsedSessionFraction(today.Timestamp)
	if err != nil || frac <= 0 {
		return nil
	}

	baseline := frac * input.AvgDailyVolume20d
	if baseline <= 0 {
		return nil
	}

	ratio := today.Volume / baseline
	return &ratio
}

// elapsedSessionFraction returns the fraction (clamped to [0,1]) of the
// standard NYSE session (9:30-16:00 America/New_York) elapsed as of t.
func elapsedSessionFraction(t time.Time) (float64, error) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return 0, fmt.Errorf("elapsedSessionFraction: failed to load America/New_York: %w", err)
	}

	et := t.In(loc)
	open := time.Date(et.Year(), et.Month(), et.Day(), nyMarketOpenHour, nyMarketOpenMinute, 0, 0, loc)
	marketClose := time.Date(et.Year(), et.Month(), et.Day(), nyMarketCloseHour, nyMarketCloseMinute, 0, 0, loc)

	sessionMinutes := marketClose.Sub(open).Minutes()
	if sessionMinutes <= 0 {
		return 0, fmt.Errorf("elapsedSessionFraction: non-positive session length")
	}

	elapsed := et.Sub(open).Minutes()
	frac := elapsed / sessionMinutes
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}

	return frac, nil
}
