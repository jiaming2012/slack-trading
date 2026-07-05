package evtracker

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/jiaming2012/slack-trading/src/go/tradingstack"
)

// setupEVTrackerDB spins up an ephemeral PostgreSQL container, opens a GORM
// connection, and migrates the trading-stack schema. The container is
// auto-terminated on test cleanup.
func setupEVTrackerDB(t *testing.T) *gorm.DB {
	t.Helper()
	ctx := context.Background()

	const (
		user   = "test"
		pass   = "test"
		dbName = "evtracker_test"
	)

	req := testcontainers.ContainerRequest{
		Image:        "postgres:13",
		ExposedPorts: []string{"5432/tcp"},
		Tmpfs:        map[string]string{"/var/lib/postgresql/data": "rw"},
		Env: map[string]string{
			"POSTGRES_USER":     user,
			"POSTGRES_PASSWORD": pass,
			"POSTGRES_DB":       dbName,
		},
		WaitingFor: wait.ForAll(
			wait.ForLog("database system is ready to accept connections"),
			wait.ForListeningPort("5432/tcp").WithStartupTimeout(60*time.Second),
		),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)
	testcontainers.CleanupContainer(t, container)

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "5432/tcp")
	require.NoError(t, err)

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		host, user, pass, dbName, port.Port())

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, tradingstack.MigrateTradingStack(db))
	return db
}

// asOfPersist is a fixed as-of used by the persistence tests, truncated to
// microsecond so timestamptz round-trips compare equal.
var asOfPersist = time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)

func before(n int) time.Time { return asOfPersist.Add(-time.Duration(n) * 24 * time.Hour) }

// resultFor finds the computed result for a (strategy, regime) group.
func resultFor(results []StrategyEVResult, strategy, regime string) (StrategyEVResult, bool) {
	for _, r := range results {
		if r.StrategyID == strategy && r.Regime == regime {
			return r, true
		}
	}
	return StrategyEVResult{}, false
}

// TestRecomputePersistsRowsPerGroup verifies each (strategy_id, regime) group
// writes exactly one row whose ev_30d, ev_90d, ev_slope, and ev_weight equal the
// engine's computed values.
func TestRecomputePersistsRowsPerGroup(t *testing.T) {
	db := setupEVTrackerDB(t)

	// Two groups, each spanning four 30-day buckets so slopes are defined.
	trades := []TradeOutcome{
		// strat-A / bull: increasing per-bucket EV
		{StrategyID: "strat-A", Regime: "bull", ClosedAt: before(100), PnL: 0.1},
		{StrategyID: "strat-A", Regime: "bull", ClosedAt: before(70), PnL: 0.3},
		{StrategyID: "strat-A", Regime: "bull", ClosedAt: before(40), PnL: 0.5},
		{StrategyID: "strat-A", Regime: "bull", ClosedAt: before(10), PnL: 0.7},
		// strat-A / bear: flat-ish
		{StrategyID: "strat-A", Regime: "bear", ClosedAt: before(80), PnL: -0.2},
		{StrategyID: "strat-A", Regime: "bear", ClosedAt: before(20), PnL: -0.2},
	}

	results, err := Recompute(context.Background(), db, NewInMemoryTradeOutcomeRepository(trades), asOfPersist, Options{})
	require.NoError(t, err)
	require.Len(t, results, 2)

	var rows []tradingstack.StrategyEvWeight
	require.NoError(t, db.Find(&rows).Error)
	require.Len(t, rows, 2, "exactly one row per group")

	for _, row := range rows {
		require.NotNil(t, row.StrategyID)
		require.NotNil(t, row.Regime)
		want, ok := resultFor(results, *row.StrategyID, *row.Regime)
		require.True(t, ok, "row references an unknown group")

		require.NotNil(t, row.ComputedAt)
		assert.True(t, asOfPersist.Equal(*row.ComputedAt), "computed_at must equal as-of")

		require.NotNil(t, row.Ev30d)
		assert.InDelta(t, want.Ev30d, *row.Ev30d, 1e-9)
		require.NotNil(t, row.Ev90d)
		assert.InDelta(t, want.Ev90d, *row.Ev90d, 1e-9)
		require.NotNil(t, row.EvWeight)
		assert.InDelta(t, want.EvWeight, *row.EvWeight, 1e-9)

		if want.EvSlope == nil {
			assert.Nil(t, row.EvSlope)
		} else {
			require.NotNil(t, row.EvSlope)
			assert.InDelta(t, *want.EvSlope, *row.EvSlope, 1e-9)
		}
	}
}

// TestRecomputeNullSlopePersistsAsNull verifies a single-bucket group persists
// ev_slope as SQL NULL with weight 0.7.
func TestRecomputeNullSlopePersistsAsNull(t *testing.T) {
	db := setupEVTrackerDB(t)

	trades := []TradeOutcome{
		{StrategyID: "strat-A", Regime: "bull", ClosedAt: before(5), PnL: 1.0},
		{StrategyID: "strat-A", Regime: "bull", ClosedAt: before(7), PnL: -1.0},
		{StrategyID: "strat-A", Regime: "bull", ClosedAt: before(9), PnL: 2.0},
	}

	_, err := Recompute(context.Background(), db, NewInMemoryTradeOutcomeRepository(trades), asOfPersist, Options{})
	require.NoError(t, err)

	var rows []tradingstack.StrategyEvWeight
	require.NoError(t, db.Find(&rows).Error)
	require.Len(t, rows, 1)
	assert.Nil(t, rows[0].EvSlope, "null slope must persist as SQL NULL")
	require.NotNil(t, rows[0].EvWeight)
	assert.InDelta(t, WeightStable, *rows[0].EvWeight, 1e-9)

	// Also assert the column is SQL NULL at the DB level.
	var nullCount int64
	require.NoError(t, db.Model(&tradingstack.StrategyEvWeight{}).
		Where("ev_slope IS NULL").Count(&nullCount).Error)
	assert.Equal(t, int64(1), nullCount)
}

// TestRecomputeRetiredProducesNoRow verifies a retired strategy is excluded
// entirely — zero rows, never a row with weight 0.
func TestRecomputeRetiredProducesNoRow(t *testing.T) {
	db := setupEVTrackerDB(t)

	trades := []TradeOutcome{
		{StrategyID: "strat-A", Regime: "bull", ClosedAt: before(5), PnL: 1.0},
		{StrategyID: "strat-A", Regime: "bull", ClosedAt: before(6), PnL: 2.0},
		{StrategyID: "strat-B", Regime: "bull", ClosedAt: before(5), PnL: -1.0},
		{StrategyID: "strat-B", Regime: "bull", ClosedAt: before(6), PnL: -2.0},
	}

	opts := Options{Retired: map[string]bool{"strat-B": true}}
	results, err := Recompute(context.Background(), db, NewInMemoryTradeOutcomeRepository(trades), asOfPersist, opts)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "strat-A", results[0].StrategyID)

	var rows []tradingstack.StrategyEvWeight
	require.NoError(t, db.Find(&rows).Error)
	require.Len(t, rows, 1)
	require.NotNil(t, rows[0].StrategyID)
	assert.Equal(t, "strat-A", *rows[0].StrategyID)

	var bCount int64
	require.NoError(t, db.Model(&tradingstack.StrategyEvWeight{}).
		Where("strategy_id = ?", "strat-B").Count(&bCount).Error)
	assert.Equal(t, int64(0), bCount, "no row may reference the retired strategy")
}

// TestRecomputeRerunReflectsNewInputs verifies that re-running for the same
// as-of with changed inputs leaves the latest computed_at rows reflecting the
// second run — no stale or duplicated weights.
func TestRecomputeRerunReflectsNewInputs(t *testing.T) {
	db := setupEVTrackerDB(t)
	ctx := context.Background()

	firstTrades := []TradeOutcome{
		{StrategyID: "strat-A", Regime: "bull", ClosedAt: before(5), PnL: 1.0},
		{StrategyID: "strat-A", Regime: "bull", ClosedAt: before(6), PnL: 1.0},
	}
	_, err := Recompute(ctx, db, NewInMemoryTradeOutcomeRepository(firstTrades), asOfPersist, Options{})
	require.NoError(t, err)

	var afterFirst []tradingstack.StrategyEvWeight
	require.NoError(t, db.Find(&afterFirst).Error)
	require.Len(t, afterFirst, 1)
	require.NotNil(t, afterFirst[0].Ev30d)
	assert.InDelta(t, 1.0, *afterFirst[0].Ev30d, 1e-9)

	// Second run: different pnls, same as-of.
	secondTrades := []TradeOutcome{
		{StrategyID: "strat-A", Regime: "bull", ClosedAt: before(5), PnL: -2.0},
		{StrategyID: "strat-A", Regime: "bull", ClosedAt: before(6), PnL: -2.0},
	}
	second, err := Recompute(ctx, db, NewInMemoryTradeOutcomeRepository(secondTrades), asOfPersist, Options{})
	require.NoError(t, err)
	require.Len(t, second, 1)

	var afterSecond []tradingstack.StrategyEvWeight
	require.NoError(t, db.Find(&afterSecond).Error)
	require.Len(t, afterSecond, 1, "re-run must not leave duplicated rows for the same as-of")
	require.NotNil(t, afterSecond[0].Ev30d)
	assert.InDelta(t, -2.0, *afterSecond[0].Ev30d, 1e-9, "latest row must reflect second run inputs")
}
