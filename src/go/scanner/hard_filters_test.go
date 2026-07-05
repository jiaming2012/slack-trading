package scanner

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// baseLiquidPassingInput returns a ScanInput that passes every Layer 1 gate
// on its own -- individual tests mutate one field at a time to isolate a
// single failure condition.
func baseLiquidPassingInput() ScanInput {
	return ScanInput{
		Ticker:             "AAPL",
		ScannedAt:          time.Date(2024, 6, 10, 15, 0, 0, 0, time.UTC),
		AvgDailyVolume20d:  800_000,
		Price:              12.50,
		MarketCap:          1_200_000_000,
		DaysToNextEarnings: intp(10),
		RecentHalt:         false,
		HasOptionsChain:    true,
		RegimeTag:          "",
	}
}

func intp(i int) *int { return &i }

// --- Liquidity gate ---

func TestLiquidityGate_PassesAllThresholdsExceeded(t *testing.T) {
	pass, reason := LiquidityGate(baseLiquidPassingInput())
	assert.True(t, pass)
	assert.Empty(t, reason)
}

func TestLiquidityGate_FailsAtVolumeFloor(t *testing.T) {
	input := baseLiquidPassingInput()
	input.AvgDailyVolume20d = 500_000 // equal to floor
	pass, reason := LiquidityGate(input)
	assert.False(t, pass)
	assert.Contains(t, reason, "avg_daily_volume_20d")
}

func TestLiquidityGate_PassesJustAboveVolumeFloor(t *testing.T) {
	input := baseLiquidPassingInput()
	input.AvgDailyVolume20d = 500_000.01
	pass, _ := LiquidityGate(input)
	assert.True(t, pass)
}

func TestLiquidityGate_FailsAtPriceFloor(t *testing.T) {
	input := baseLiquidPassingInput()
	input.Price = 5.00 // equal to floor
	pass, reason := LiquidityGate(input)
	assert.False(t, pass)
	assert.Contains(t, reason, "price")
}

func TestLiquidityGate_PassesJustAbovePriceFloor(t *testing.T) {
	input := baseLiquidPassingInput()
	input.Price = 5.01
	pass, _ := LiquidityGate(input)
	assert.True(t, pass)
}

func TestLiquidityGate_FailsAtMarketCapFloor(t *testing.T) {
	input := baseLiquidPassingInput()
	input.MarketCap = 300_000_000 // equal to floor
	pass, reason := LiquidityGate(input)
	assert.False(t, pass)
	assert.Contains(t, reason, "market_cap")
}

func TestLiquidityGate_PassesJustAboveMarketCapFloor(t *testing.T) {
	input := baseLiquidPassingInput()
	input.MarketCap = 300_000_001
	pass, _ := LiquidityGate(input)
	assert.True(t, pass)
}

// --- Data quality gate ---

func TestDataQualityGate_PassesAllConditions(t *testing.T) {
	pass, reason := DataQualityGate(baseLiquidPassingInput())
	assert.True(t, pass)
	assert.Empty(t, reason)
}

func TestDataQualityGate_FailsWhenEarningsWithinWindow(t *testing.T) {
	input := baseLiquidPassingInput()
	input.DaysToNextEarnings = intp(2)
	pass, reason := DataQualityGate(input)
	assert.False(t, pass)
	assert.Contains(t, reason, "earnings")
}

func TestDataQualityGate_FailsAtEarningsWindowBoundary(t *testing.T) {
	input := baseLiquidPassingInput()
	input.DaysToNextEarnings = intp(3) // exactly at the 3-day boundary
	pass, _ := DataQualityGate(input)
	assert.False(t, pass)
}

func TestDataQualityGate_PassesJustOutsideEarningsWindow(t *testing.T) {
	input := baseLiquidPassingInput()
	input.DaysToNextEarnings = intp(4)
	pass, _ := DataQualityGate(input)
	assert.True(t, pass)
}

func TestDataQualityGate_FailsForPastEarningsWithinWindow(t *testing.T) {
	input := baseLiquidPassingInput()
	input.DaysToNextEarnings = intp(-3) // earnings 3 days ago, boundary
	pass, _ := DataQualityGate(input)
	assert.False(t, pass)
}

func TestDataQualityGate_PassesForPastEarningsOutsideWindow(t *testing.T) {
	input := baseLiquidPassingInput()
	input.DaysToNextEarnings = intp(-4)
	pass, _ := DataQualityGate(input)
	assert.True(t, pass)
}

func TestDataQualityGate_FailsOnRecentHalt(t *testing.T) {
	input := baseLiquidPassingInput()
	input.RecentHalt = true
	pass, reason := DataQualityGate(input)
	assert.False(t, pass)
	assert.Contains(t, reason, "halt")
}

func TestDataQualityGate_FailsWhenNoOptionsChain(t *testing.T) {
	input := baseLiquidPassingInput()
	input.HasOptionsChain = false
	pass, reason := DataQualityGate(input)
	assert.False(t, pass)
	assert.Contains(t, reason, "options chain")
}

// --- Regime filter ---

// flatCandlesWithATR builds an ascending candle series whose ATR% is
// controllable via dailyRange, and whose price_vs_50ma is ~0 (won't trip the
// MA bound), isolating the ATR% condition for regime tests.
func flatCandlesWithATR(t *testing.T, closes []float64, dailyRange float64) []Candle {
	t.Helper()
	candles := make([]Candle, len(closes))
	start := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	for i, c := range closes {
		candles[i] = Candle{
			Timestamp: start.AddDate(0, 0, i),
			Open:      c,
			High:      c + dailyRange/2,
			Low:       c - dailyRange/2,
			Close:     c,
			Volume:    1_000_000,
		}
	}
	return candles
}

func TestRegimeFilter_HighVolRejectsAboveTighterATRCeiling(t *testing.T) {
	// 15 flat closes at 100 with a daily range of 5.0 (each day's High/Low
	// straddle the constant close by +/-2.5), giving true range = 5.0 every
	// day, so atr_pct = ATR14/price = 5.0/100 = 0.05 -- above the high_vol
	// ceiling (0.04) but at/below the trending ceiling (0.10).
	closes := make([]float64, 15)
	for i := range closes {
		closes[i] = 100
	}
	candles := flatCandlesWithATR(t, closes, 5.0)

	input := baseLiquidPassingInput()
	input.Candles = candles
	input.Price = 100
	input.RegimeTag = "high_vol"

	pass, reason := RegimeFilter(input)
	assert.False(t, pass)
	assert.Contains(t, reason, "atr_pct")
}

func TestRegimeFilter_SameATRPassesUnderLooserRegime(t *testing.T) {
	closes := make([]float64, 15)
	for i := range closes {
		closes[i] = 100
	}
	candles := flatCandlesWithATR(t, closes, 5.0) // same atr_pct = 0.05 as above

	input := baseLiquidPassingInput()
	input.Candles = candles
	input.Price = 100
	input.RegimeTag = "trending" // looser ceiling (0.10) than high_vol (0.04)

	pass, _ := RegimeFilter(input)
	assert.True(t, pass)
}

func TestRegimeFilter_MissingRegimeTagIsNoOp(t *testing.T) {
	closes := make([]float64, 15)
	for i := range closes {
		closes[i] = 100
	}
	// Deliberately extreme ATR (would fail every known regime).
	candles := flatCandlesWithATR(t, closes, 50.0)

	input := baseLiquidPassingInput()
	input.Candles = candles
	input.Price = 100
	input.RegimeTag = "" // empty -> no-op

	pass, _ := RegimeFilter(input)
	assert.True(t, pass)
}

func TestRegimeFilter_UnrecognizedRegimeTagIsNoOp(t *testing.T) {
	closes := make([]float64, 15)
	for i := range closes {
		closes[i] = 100
	}
	candles := flatCandlesWithATR(t, closes, 50.0)

	input := baseLiquidPassingInput()
	input.Candles = candles
	input.Price = 100
	input.RegimeTag = "some_unknown_regime"

	pass, _ := RegimeFilter(input)
	assert.True(t, pass)
}

func TestRegimeFilter_PassesAtExactCeilingBoundary(t *testing.T) {
	// atr_pct exactly at the high_vol ceiling (0.04) is not "exceeding" it.
	closes := make([]float64, 15)
	for i := range closes {
		closes[i] = 100
	}
	candles := flatCandlesWithATR(t, closes, 4.0) // atr_pct = 4/100 = 0.04
	input := baseLiquidPassingInput()
	input.Candles = candles
	input.Price = 100
	input.RegimeTag = "high_vol"

	pass, _ := RegimeFilter(input)
	assert.True(t, pass, "atr_pct exactly at the ceiling should pass, only exceeding it fails")
}

// --- RunLayer1 aggregation ---

func TestRunLayer1_PassingTickerHasNoReasons(t *testing.T) {
	result := RunLayer1(baseLiquidPassingInput())
	assert.True(t, result.Pass)
	assert.Empty(t, result.Reasons)
}

func TestRunLayer1_SingleGateFailureReported(t *testing.T) {
	input := baseLiquidPassingInput()
	input.AvgDailyVolume20d = 100_000 // fails liquidity only

	result := RunLayer1(input)
	assert.False(t, result.Pass)
	assert.Len(t, result.Reasons, 1)
}

func TestRunLayer1_MultipleGateFailuresAllReported(t *testing.T) {
	input := baseLiquidPassingInput()
	input.Price = 2.0        // fails liquidity (price too low)
	input.RecentHalt = true  // fails data quality (recent halt)

	result := RunLayer1(input)
	assert.False(t, result.Pass)
	assert.Len(t, result.Reasons, 2, "both failing gates must be reported, not just the first")
}
