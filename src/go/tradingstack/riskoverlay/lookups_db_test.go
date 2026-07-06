package riskoverlay

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/jiaming2012/slack-trading/src/go/tradingstack"
	"github.com/jiaming2012/slack-trading/src/go/tradingstack/crowding"
)

// setupRiskOverlayDB boots an ephemeral postgres:13 container and migrates the
// trading-stack tables (strategy_ev_weights, scan_results) plus the crowding
// tables, mirroring the tradingstack testcontainers pattern. Requires Docker.
func setupRiskOverlayDB(t *testing.T) *gorm.DB {
	t.Helper()
	ctx := context.Background()

	const (
		user   = "test"
		pass   = "test"
		dbName = "riskoverlay_test"
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
	require.NoError(t, crowding.MigrateCrowdingDetection(db))

	return db
}

func dbStrp(s string) *string        { return &s }
func dbF64p(f float64) *float64      { return &f }
func dbTimep(v time.Time) *time.Time { return &v }

// GormEvWeightLookup returns the MOST RECENT ev_weight per strategy_id and
// skips rows with NULL strategy_id or NULL ev_weight.
func TestGormEvWeightLookup_RoundTrip(t *testing.T) {
	db := setupRiskOverlayDB(t)

	older := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)

	rows := []tradingstack.StrategyEvWeight{
		{ComputedAt: dbTimep(older), StrategyID: dbStrp("cc-v7"), EvWeight: dbF64p(0.10)},
		{ComputedAt: dbTimep(newer), StrategyID: dbStrp("cc-v7"), EvWeight: dbF64p(0.75)}, // latest wins
		{ComputedAt: dbTimep(newer), StrategyID: dbStrp("wheel"), EvWeight: dbF64p(0.25)},
		{ComputedAt: dbTimep(newer), StrategyID: dbStrp("null-weight"), EvWeight: nil}, // skipped
		{ComputedAt: dbTimep(newer), StrategyID: nil, EvWeight: dbF64p(0.5)},           // skipped
	}
	for i := range rows {
		require.NoError(t, db.Create(&rows[i]).Error)
	}

	weights, err := NewGormEvWeightLookup(db).Latest()
	require.NoError(t, err)
	require.Equal(t, map[string]float64{"cc-v7": 0.75, "wheel": 0.25}, weights)
}

// An empty strategy_ev_weights table yields an empty map with no error (the
// family pins inactive downstream — that is a data-availability state, not a
// lookup failure).
func TestGormEvWeightLookup_EmptyTable(t *testing.T) {
	db := setupRiskOverlayDB(t)

	weights, err := NewGormEvWeightLookup(db).Latest()
	require.NoError(t, err)
	require.Empty(t, weights)
}

// GormSectorLookup returns the LATEST non-null sector per ticker and the empty
// sector (no error) for an unknown ticker.
func TestGormSectorLookup_RoundTrip(t *testing.T) {
	db := setupRiskOverlayDB(t)

	older := time.Date(2026, 7, 1, 14, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 7, 4, 14, 0, 0, 0, time.UTC)

	results := []tradingstack.ScanResult{
		{ScannedAt: older, Ticker: "AAPL", Sector: dbStrp("legacy-tech")},
		{ScannedAt: newer, Ticker: "AAPL", Sector: dbStrp("technology")}, // latest wins
		{ScannedAt: newer.Add(time.Hour), Ticker: "AAPL", Sector: nil},   // NULL sector ignored
		{ScannedAt: newer, Ticker: "KO", Sector: dbStrp("staples")},
	}
	for i := range results {
		require.NoError(t, db.Create(&results[i]).Error)
	}

	lookup := NewGormSectorLookup(db)

	sector, err := lookup.SectorOf("AAPL")
	require.NoError(t, err)
	require.Equal(t, "technology", sector)

	sector, err = lookup.SectorOf("KO")
	require.NoError(t, err)
	require.Equal(t, "staples", sector)

	sector, err = lookup.SectorOf("ZZZQ")
	require.NoError(t, err)
	require.Equal(t, "", sector, "unknown ticker resolves to the empty sector with no error")
}

// GormCrowdingLookup resolves the cycle IN EFFECT at the given time: the most
// recent metric at or before it. An exact scanned_at still resolves to its own
// cycle, and a time before every cycle yields an unflagged empty view.
func TestGormCrowdingLookup_LatestAtOrBefore(t *testing.T) {
	db := setupRiskOverlayDB(t)

	cycle1 := time.Date(2026, 7, 4, 14, 0, 0, 0, time.UTC)
	cycle2 := time.Date(2026, 7, 4, 15, 0, 0, 0, time.UTC)

	scan := tradingstack.ScanResult{ScannedAt: cycle2, Ticker: "NVDA", Sector: dbStrp("technology")}
	require.NoError(t, db.Create(&scan).Error)

	m1 := crowding.CrowdingMetric{ScannedAt: cycle1, ComputedAt: cycle1, TotalCandidates: 10, OverlappingCandidates: 1, OverlapPct: 10, ThresholdPct: 50, Flagged: false}
	require.NoError(t, db.Create(&m1).Error)

	m2 := crowding.CrowdingMetric{ScannedAt: cycle2, ComputedAt: cycle2, TotalCandidates: 10, OverlappingCandidates: 8, OverlapPct: 80, ThresholdPct: 50, Flagged: true}
	require.NoError(t, db.Create(&m2).Error)

	cand := crowding.CrowdingFlaggedCandidate{CrowdingMetricID: m2.ID, ScanResultID: scan.ID, Ticker: "NVDA", StrategyIDs: crowding.JSONArray{"cc-v7"}}
	require.NoError(t, db.Create(&cand).Error)

	lookup := NewGormCrowdingLookup(db)

	t.Run("a later wall-clock time resolves the most recent cycle", func(t *testing.T) {
		view, err := lookup.ViewForScanCycle(cycle2.Add(20 * time.Minute))
		require.NoError(t, err)
		require.True(t, view.Flagged)
		require.True(t, view.IsFlagged("NVDA"))
	})

	t.Run("an exact scanned_at resolves its own cycle", func(t *testing.T) {
		view, err := lookup.ViewForScanCycle(cycle1)
		require.NoError(t, err)
		require.False(t, view.Flagged)
		require.Empty(t, view.Tickers())
	})

	t.Run("a time before every cycle yields an unflagged empty view", func(t *testing.T) {
		view, err := lookup.ViewForScanCycle(cycle1.Add(-time.Hour))
		require.NoError(t, err)
		require.False(t, view.Flagged)
		require.Empty(t, view.Tickers())
	})
}
