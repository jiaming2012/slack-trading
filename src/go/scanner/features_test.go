package scanner

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const tol = 1e-6

// --- rsi_14 ---

// TestComputeRSI14_MatchesHandComputedFixture uses 15 closes with 14 diffs
// alternating +2/-1 (7 of each): avgGain = 7*2/14 = 1, avgLoss = 7*1/14 = 0.5,
// RS = 2, RSI = 100 - 100/(1+2) = 100 - 33.333... = 66.6666...67 (= 200/3).
func TestComputeRSI14_MatchesHandComputedFixture(t *testing.T) {
	closes := []float64{50, 52, 51, 53, 52, 54, 53, 55, 54, 56, 55, 57, 56, 58, 57}
	got := ComputeRSI14(closes)
	if assert.NotNil(t, got) {
		assert.InDelta(t, 200.0/3.0, *got, 1e-9)
	}
}

func TestComputeRSI14_AllGainsIsHundred(t *testing.T) {
	// 15 closes, each +1 vs the previous: avgLoss = 0, avgGain != 0 -> RSI 100.
	closes := make([]float64, 15)
	for i := range closes {
		closes[i] = 100 + float64(i)
	}
	got := ComputeRSI14(closes)
	if assert.NotNil(t, got) {
		assert.InDelta(t, 100.0, *got, tol)
	}
}

func TestComputeRSI14_InsufficientHistoryYieldsNil(t *testing.T) {
	closes := make([]float64, 14) // one short of the required 15
	for i := range closes {
		closes[i] = 100 + float64(i)
	}
	assert.Nil(t, ComputeRSI14(closes))
}

func TestComputeRSI14_ExactlyMinimumHistoryComputes(t *testing.T) {
	closes := make([]float64, 15) // exactly the required minimum
	for i := range closes {
		closes[i] = 100 + float64(i)
	}
	assert.NotNil(t, ComputeRSI14(closes))
}

// --- atr_pct ---

// buildATRFixtureCandles builds 15 candles where Close increases by 1 each
// day and each day's High/Low are Close+1/Close-1 (a constant range of 2).
// For every day after the first: High-Low = 2, |High-prevClose| = 2,
// |Low-prevClose| = 0, so TR = 2 every day. ATR14 = 2 exactly.
func buildATRFixtureCandles() []Candle {
	candles := make([]Candle, 15)
	start := time.Date(2024, 3, 1, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 15; i++ {
		closePrice := 100 + float64(i)
		candles[i] = Candle{
			Timestamp: start.AddDate(0, 0, i),
			Open:      closePrice,
			High:      closePrice + 1,
			Low:       closePrice - 1,
			Close:     closePrice,
			Volume:    1_000_000,
		}
	}
	return candles
}

func TestComputeATRPct_MatchesHandComputedFixture(t *testing.T) {
	candles := buildATRFixtureCandles()
	price := candles[len(candles)-1].Close // 114
	got := ComputeATRPct(candles, price)
	if assert.NotNil(t, got) {
		// ATR14 = 2, price = 114 -> atr_pct = 2/114
		assert.InDelta(t, 2.0/114.0, *got, 1e-9)
	}
}

func TestComputeATRPct_InsufficientHistoryYieldsNil(t *testing.T) {
	candles := buildATRFixtureCandles()[1:] // 14 candles, one short
	assert.Nil(t, ComputeATRPct(candles, 100))
}

func TestComputeATRPct_NonPositivePriceYieldsNil(t *testing.T) {
	candles := buildATRFixtureCandles()
	assert.Nil(t, ComputeATRPct(candles, 0))
}

// --- price_vs_50ma ---

// TestComputePriceVs50MA_MatchesHandComputedFixture uses 50 closes
// 100..149 (arithmetic sequence, step 1): MA50 = (100+149)/2 = 124.5.
// price = 149 (the latest close) -> (149-124.5)/124.5*100 = 24.5/124.5*100.
func TestComputePriceVs50MA_MatchesHandComputedFixture(t *testing.T) {
	closes := make([]float64, 50)
	for i := range closes {
		closes[i] = 100 + float64(i)
	}
	price := 149.0
	got := ComputePriceVs50MA(closes, price)
	if assert.NotNil(t, got) {
		want := (24.5 / 124.5) * 100
		assert.InDelta(t, want, *got, 1e-9)
	}
}

func TestComputePriceVs50MA_InsufficientHistoryYieldsNil(t *testing.T) {
	closes := make([]float64, 49) // one short of the required 50
	for i := range closes {
		closes[i] = 100 + float64(i)
	}
	assert.Nil(t, ComputePriceVs50MA(closes, 149))
}

func TestComputePriceVs50MA_ExactlyMinimumHistoryComputes(t *testing.T) {
	closes := make([]float64, 50)
	for i := range closes {
		closes[i] = 100 + float64(i)
	}
	assert.NotNil(t, ComputePriceVs50MA(closes, 149))
}

// --- compression_score ---

// TestComputeCompressionScore_MatchesHandComputedFixture uses 9 closes with
// period 5: the first four rolling 5-windows are all flat at 10 (width 0),
// and the current (5th) window is [10,10,10,10,20] (mean 12, population
// variance = (4*4+64)/5 = 16, std = 4, width = 4*std/mean = 16/12).
// Since the current width (16/12) exceeds every one of the four historical
// widths (all 0), the percentile rank is 100.
func TestComputeCompressionScore_MatchesHandComputedFixture(t *testing.T) {
	closes := []float64{10, 10, 10, 10, 10, 10, 10, 10, 20}
	got := ComputeCompressionScore(closes)
	if assert.NotNil(t, got) {
		assert.InDelta(t, 100.0, *got, tol)
	}
}

func TestComputeCompressionScore_InsufficientHistoryYieldsNil(t *testing.T) {
	closes := []float64{10, 10, 10, 10, 10} // exactly period, no historical point
	assert.Nil(t, ComputeCompressionScore(closes))
}

// --- volume_ratio ---

// nyDate builds a time.Time at the given hour/minute in America/New_York on
// a fixed date (Jan 2 2024, a Tuesday, standard time / no DST ambiguity).
func nyDate(t *testing.T, hour, minute int) time.Time {
	t.Helper()
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("failed to load America/New_York: %v", err)
	}
	return time.Date(2024, 1, 2, hour, minute, 0, 0, loc)
}

// TestComputeVolumeRatio_NearOneForPaceTypicalVolume: 30 minutes into the
// session (9:30-10:00), elapsed fraction = 30/390. With AvgDailyVolume20d =
// 390,000, baseline = 30/390*390,000 = 30,000 exactly. Today's volume-so-far
// set to exactly 30,000 -> ratio should be ~1.0.
func TestComputeVolumeRatio_NearOneForPaceTypicalVolume(t *testing.T) {
	input := ScanInput{
		AvgDailyVolume20d: 390_000,
		Candles: []Candle{
			{Timestamp: nyDate(t, 10, 0), Close: 100, Volume: 30_000},
		},
	}
	got := ComputeVolumeRatio(input)
	if assert.NotNil(t, got) {
		assert.InDelta(t, 1.0, *got, 1e-9)
	}
}

// TestComputeVolumeRatio_FlagsGenuineEarlySessionSpike: same setup, but
// today's volume-so-far is double the pace-adjusted baseline -> ratio ~2.0.
func TestComputeVolumeRatio_FlagsGenuineEarlySessionSpike(t *testing.T) {
	input := ScanInput{
		AvgDailyVolume20d: 390_000,
		Candles: []Candle{
			{Timestamp: nyDate(t, 10, 0), Close: 100, Volume: 60_000},
		},
	}
	got := ComputeVolumeRatio(input)
	if assert.NotNil(t, got) {
		assert.InDelta(t, 2.0, *got, 1e-9)
	}
}

func TestComputeVolumeRatio_NoCandlesYieldsNil(t *testing.T) {
	input := ScanInput{AvgDailyVolume20d: 390_000}
	assert.Nil(t, ComputeVolumeRatio(input))
}

func TestComputeVolumeRatio_NonPositiveAvgVolumeYieldsNil(t *testing.T) {
	input := ScanInput{
		AvgDailyVolume20d: 0,
		Candles: []Candle{
			{Timestamp: nyDate(t, 10, 0), Close: 100, Volume: 30_000},
		},
	}
	assert.Nil(t, ComputeVolumeRatio(input))
}

// --- pass-through features ---

func TestPassThroughFeatures_RecordedVerbatim(t *testing.T) {
	shortInterest := 0.18
	sectorMomentum := 0.04

	input := ScanInput{
		Ticker:            "AAPL",
		ScannedAt:         time.Date(2024, 6, 10, 15, 0, 0, 0, time.UTC),
		Price:             100,
		ShortInterest:     &shortInterest,
		SectorMomentum10d: &sectorMomentum,
		RegimeTag:         "trending",
		ScannerVersion:    "l1l2-v1",
	}

	fv, err := BuildFeatureVector(input)
	assert.NoError(t, err)
	if assert.NotNil(t, fv.ShortInterest) {
		assert.Equal(t, 0.18, *fv.ShortInterest)
	}
	if assert.NotNil(t, fv.SectorMomentum10d) {
		assert.Equal(t, 0.04, *fv.SectorMomentum10d)
	}
	assert.Equal(t, "trending", fv.RegimeTag)
}

// --- data_as_of accuracy invariant ---

func TestBuildFeatureVector_ValidDataAsOfProducesVector(t *testing.T) {
	scannedAt := time.Date(2024, 6, 10, 15, 0, 0, 0, time.UTC)
	lastCandleTime := scannedAt.Add(-time.Hour) // strictly before scanned_at

	input := ScanInput{
		Ticker:    "AAPL",
		ScannedAt: scannedAt,
		Price:     100,
		Candles: []Candle{
			{Timestamp: lastCandleTime, Close: 100, Volume: 1000},
		},
	}

	fv, err := BuildFeatureVector(input)
	assert.NoError(t, err)
	if assert.NotNil(t, fv) {
		assert.True(t, fv.DataAsOf.Equal(lastCandleTime))
	}
}

func TestBuildFeatureVector_DataAsOfEqualToScannedAtPasses(t *testing.T) {
	scannedAt := time.Date(2024, 6, 10, 15, 0, 0, 0, time.UTC)

	input := ScanInput{
		Ticker:    "AAPL",
		ScannedAt: scannedAt,
		Price:     100,
		Candles: []Candle{
			{Timestamp: scannedAt, Close: 100, Volume: 1000}, // exactly equal, boundary
		},
	}

	fv, err := BuildFeatureVector(input)
	assert.NoError(t, err)
	assert.NotNil(t, fv)
}

func TestBuildFeatureVector_DataAsOfAfterScannedAtFailsClosed(t *testing.T) {
	scannedAt := time.Date(2024, 6, 10, 15, 0, 0, 0, time.UTC)
	lastCandleTime := scannedAt.Add(time.Nanosecond) // strictly after, boundary

	input := ScanInput{
		Ticker:    "AAPL",
		ScannedAt: scannedAt,
		Price:     100,
		Candles: []Candle{
			{Timestamp: lastCandleTime, Close: 100, Volume: 1000},
		},
	}

	fv, err := BuildFeatureVector(input)
	assert.Nil(t, fv)
	assert.ErrorIs(t, err, ErrDataAsOfAfterScannedAt)
}
