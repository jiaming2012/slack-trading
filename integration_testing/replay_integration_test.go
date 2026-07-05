//go:build integration

package integrationtesting

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jiaming2012/slack-trading/src/go/backtester/models"
	eventmodels "github.com/jiaming2012/slack-trading/src/go/models"
)

// TestReplayMatchesInMemory proves that signals written to ESDB and read back
// via the ESDBSignalRepository produce identical clock-gated delivery as the
// InMemorySignalRepository baseline. This is the core "dual-run" test for the
// replay feature (REPLAY-02).
func TestReplayMatchesInMemory(t *testing.T) {
	if testing.Short() {
		t.Skip("requires docker")
	}

	ctx := context.Background()

	// ---- Setup: ESDB container and producer ----
	connStr := startESDBContainer(t, ctx)
	producer, cancel := createESDBProducer(t, connStr)
	defer cancel()

	// ---- Define test signals spanning a 45-minute window ----
	t0 := time.Date(2025, 3, 15, 10, 0, 0, 0, time.UTC)

	testSignals := []*eventmodels.TradeSignal{
		eventmodels.NewTradeSignal(eventmodels.SignalMACrossover, "AAPL", t0, map[string]interface{}{"direction": "up"}),
		eventmodels.NewTradeSignal(eventmodels.SignalMeanReversion, "MSFT", t0.Add(5*time.Minute), map[string]interface{}{"zscore": "1.8"}),
		eventmodels.NewTradeSignal(eventmodels.SignalCoveredCall, "AAPL", t0.Add(10*time.Minute), map[string]interface{}{"delta": "0.30"}),
		eventmodels.NewTradeSignal(eventmodels.SignalStartOfWeek, "SPY", t0.Add(15*time.Minute), map[string]interface{}{"week": "12"}),
		eventmodels.NewTradeSignal(eventmodels.SignalMACrossover, "MSFT", t0.Add(20*time.Minute), map[string]interface{}{"direction": "down"}),
		eventmodels.NewTradeSignal(eventmodels.SignalMeanReversion, "AAPL", t0.Add(25*time.Minute), map[string]interface{}{"zscore": "-2.3"}),
		eventmodels.NewTradeSignal(eventmodels.SignalCoveredCall, "SPY", t0.Add(30*time.Minute), map[string]interface{}{"delta": "0.25"}),
		eventmodels.NewTradeSignal(eventmodels.SignalMACrossover, "AAPL", t0.Add(35*time.Minute), map[string]interface{}{"direction": "up"}),
		eventmodels.NewTradeSignal(eventmodels.SignalStartOfWeek, "QQQ", t0.Add(40*time.Minute), map[string]interface{}{"week": "13"}),
		eventmodels.NewTradeSignal(eventmodels.SignalMeanReversion, "MSFT", t0.Add(45*time.Minute), map[string]interface{}{"zscore": "3.1"}),
	}

	// ---- Run 1: In-memory baseline ----
	memRepo := models.NewInMemorySignalRepository()
	for _, sig := range testSignals {
		require.NoError(t, memRepo.Write(sig))
	}

	type tickResult struct {
		tickTime time.Time
		signals  []*eventmodels.TradeSignal
	}

	simulateClock := func(repo *models.InMemorySignalRepository) []tickResult {
		var results []tickResult
		// Advance clock from T0 through T0+45m in 5-minute increments
		for offset := time.Duration(0); offset <= 45*time.Minute; offset += 5 * time.Minute {
			currentTime := t0.Add(offset)
			pending := repo.ReadPending(currentTime)
			results = append(results, tickResult{
				tickTime: currentTime,
				signals:  pending,
			})
		}
		return results
	}

	baselineResults := simulateClock(memRepo)

	// ---- Persist signals to ESDB ----
	esdbRepo := models.NewESDBSignalRepository(producer)
	for _, sig := range testSignals {
		require.NoError(t, esdbRepo.Write(sig))
	}

	// ---- Run 2: ESDB replay ----
	// Read all signals back from ESDB
	replaySignals := esdbRepo.GetAll()
	require.Len(t, replaySignals, len(testSignals), "ESDB should return all written signals")

	// Load replayed signals into a fresh InMemorySignalRepository
	replayMemRepo := models.NewInMemorySignalRepository()
	for _, sig := range replaySignals {
		require.NoError(t, replayMemRepo.Write(sig))
	}

	replayResults := simulateClock(replayMemRepo)

	// ---- Compare: dual-run results must match exactly ----
	require.Equal(t, len(baselineResults), len(replayResults), "tick count must match")

	totalBaselineSignals := 0
	totalReplaySignals := 0

	for i := range baselineResults {
		baseline := baselineResults[i]
		replay := replayResults[i]

		assert.Equal(t, baseline.tickTime, replay.tickTime, "tick %d: time must match", i)
		assert.Len(t, replay.signals, len(baseline.signals), "tick %d (%s): signal count must match", i, baseline.tickTime.Format(time.RFC3339))

		totalBaselineSignals += len(baseline.signals)
		totalReplaySignals += len(replay.signals)

		// Compare each signal delivered at this tick
		for j := range baseline.signals {
			if j >= len(replay.signals) {
				break
			}
			bSig := baseline.signals[j]
			rSig := replay.signals[j]

			assert.Equal(t, bSig.Name, rSig.Name, "tick %d signal %d: Name must match", i, j)
			assert.Equal(t, bSig.Symbol, rSig.Symbol, "tick %d signal %d: Symbol must match", i, j)
			assert.True(t, bSig.Timestamp.Equal(rSig.Timestamp), "tick %d signal %d: Timestamp must match (baseline=%s, replay=%s)", i, j, bSig.Timestamp, rSig.Timestamp)
			assert.Equal(t, bSig.Attributes, rSig.Attributes, "tick %d signal %d: Attributes must match", i, j)
		}
	}

	// Verify totals
	assert.Equal(t, len(testSignals), totalBaselineSignals, "baseline must deliver all signals")
	assert.Equal(t, len(testSignals), totalReplaySignals, "replay must deliver all signals")
	assert.Equal(t, totalBaselineSignals, totalReplaySignals, "total delivered signal count must match between runs")
}
