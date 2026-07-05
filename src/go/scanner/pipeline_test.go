package scanner

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

// setupScannerDB spins up an ephemeral PostgreSQL container, opens a GORM
// connection, and runs the trading-stack schema migration (mirroring the
// trading-stack-schema package's own test pattern). The container is
// auto-terminated on test cleanup.
func setupScannerDB(t *testing.T) *gorm.DB {
	t.Helper()
	ctx := context.Background()

	const (
		user   = "test"
		pass   = "test"
		dbName = "scanner_test"
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

// qualifyingInput returns a ScanInput that passes every Layer 1 gate and has
// sufficient history for every Layer 2 feature (60 ascending daily candles
// plus enough headroom for RSI/ATR/MA/compression, with the last candle
// standing in for the current, still-in-progress session).
func qualifyingInput(t *testing.T, ticker string, scannedAt time.Time) ScanInput {
	t.Helper()

	const n = 60
	candles := make([]Candle, n)
	start := scannedAt.AddDate(0, 0, -n)
	for i := 0; i < n; i++ {
		closePrice := 100 + float64(i)*0.5
		candles[i] = Candle{
			Timestamp: start.AddDate(0, 0, i),
			Open:      closePrice,
			High:      closePrice + 1,
			Low:       closePrice - 1,
			Close:     closePrice,
			Volume:    1_000_000,
		}
	}
	// Last candle stands in for "today", timestamped at scannedAt so
	// data_as_of <= scanned_at holds.
	candles[n-1].Timestamp = scannedAt

	shortInterest := 0.12
	sectorMomentum := 0.03

	return ScanInput{
		Ticker:             ticker,
		ScannedAt:          scannedAt,
		Candles:            candles,
		Price:              candles[n-1].Close,
		AvgDailyVolume20d:  800_000,
		MarketCap:          1_200_000_000,
		DaysToNextEarnings: intp(30),
		RecentHalt:         false,
		HasOptionsChain:    true,
		ShortInterest:      &shortInterest,
		SectorMomentum10d:  &sectorMomentum,
		RegimeTag:          "trending",
		ScannerVersion:     "l1l2-v1",
	}
}

func TestRunScan_QualifyingTickerProducesExactlyOneFullyPopulatedRow(t *testing.T) {
	db := setupScannerDB(t)
	scannedAt := time.Date(2024, 6, 10, 15, 0, 0, 0, time.UTC)
	input := qualifyingInput(t, "AAPL", scannedAt)

	sr, err := RunScan(db, input)
	require.NoError(t, err)
	require.NotNil(t, sr)

	var count int64
	require.NoError(t, db.Model(&tradingstack.ScanResult{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)

	var got tradingstack.ScanResult
	require.NoError(t, db.First(&got, "id = ?", sr.ID).Error)

	assert.Equal(t, "AAPL", got.Ticker)
	require.NotNil(t, got.RegimeTag)
	assert.Equal(t, "trending", *got.RegimeTag)
	require.NotNil(t, got.Price)
	assert.InDelta(t, input.Price, *got.Price, 1e-9)
	require.NotNil(t, got.VolumeRatio)
	require.NotNil(t, got.Rsi14)
	require.NotNil(t, got.AtrPct)
	require.NotNil(t, got.ShortInterest)
	assert.InDelta(t, 0.12, *got.ShortInterest, 1e-9)
	require.NotNil(t, got.ScannerVersion)
	assert.Equal(t, "l1l2-v1", *got.ScannerVersion)
	require.NotNil(t, got.DataAsOf)
	assert.True(t, got.DataAsOf.Equal(scannedAt))
}

func TestRunScan_Layer1RejectedTickerProducesNoRowAndNoFeatureComputation(t *testing.T) {
	db := setupScannerDB(t)
	scannedAt := time.Date(2024, 6, 10, 15, 0, 0, 0, time.UTC)
	input := qualifyingInput(t, "PENNY", scannedAt)
	input.Price = 1.0 // fails the liquidity gate

	sr, err := RunScan(db, input)
	assert.Nil(t, sr)
	require.Error(t, err)

	var filteredErr ErrFilteredOut
	require.ErrorAs(t, err, &filteredErr)
	assert.NotEmpty(t, filteredErr.Reasons)

	var count int64
	require.NoError(t, db.Model(&tradingstack.ScanResult{}).Count(&count).Error)
	assert.Equal(t, int64(0), count, "a Layer-1-rejected ticker must produce no scan_results row")
}

func TestRunScan_DataAsOfAfterScannedAtFailsClosedWithNoRow(t *testing.T) {
	db := setupScannerDB(t)
	scannedAt := time.Date(2024, 6, 10, 15, 0, 0, 0, time.UTC)
	input := qualifyingInput(t, "AAPL", scannedAt)
	// Push the "today" candle's timestamp strictly after scanned_at.
	input.Candles[len(input.Candles)-1].Timestamp = scannedAt.Add(time.Hour)

	sr, err := RunScan(db, input)
	assert.Nil(t, sr)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrDataAsOfAfterScannedAt)

	var count int64
	require.NoError(t, db.Model(&tradingstack.ScanResult{}).Count(&count).Error)
	assert.Equal(t, int64(0), count, "no scan_results row should be written on a data_as_of violation")
}

func TestRunScan_RerunDoesNotMutateFirstRow(t *testing.T) {
	db := setupScannerDB(t)
	scannedAt := time.Date(2024, 6, 10, 15, 0, 0, 0, time.UTC)
	input := qualifyingInput(t, "AAPL", scannedAt)

	first, err := RunScan(db, input)
	require.NoError(t, err)

	// Re-run for the same ticker/scanned_at with different input values
	// (kept within the regime filter's bounds so the re-run still qualifies).
	secondInput := input
	newShortInterest := 0.99
	secondInput.ShortInterest = &newShortInterest
	secondInput.Price = input.Price + 1

	_, err = RunScan(db, secondInput)
	require.NoError(t, err)

	var count int64
	require.NoError(t, db.Model(&tradingstack.ScanResult{}).Count(&count).Error)
	assert.Equal(t, int64(2), count, "a re-run must produce a new, independent row, never an update")

	var reloadedFirst tradingstack.ScanResult
	require.NoError(t, db.First(&reloadedFirst, "id = ?", first.ID).Error)
	require.NotNil(t, reloadedFirst.ShortInterest)
	assert.InDelta(t, 0.12, *reloadedFirst.ShortInterest, 1e-9, "the first row's stored values must be unchanged after the second run")
	require.NotNil(t, reloadedFirst.Price)
	assert.InDelta(t, input.Price, *reloadedFirst.Price, 1e-9)
}

func TestRunScan_DifferentScannerVersionsTagDistinctly(t *testing.T) {
	db := setupScannerDB(t)
	scannedAt := time.Date(2024, 6, 10, 15, 0, 0, 0, time.UTC)

	inputV1 := qualifyingInput(t, "AAPL", scannedAt)
	inputV1.ScannerVersion = "l1l2-v1"

	inputV2 := qualifyingInput(t, "MSFT", scannedAt)
	inputV2.ScannerVersion = "l1l2-v2"

	srV1, err := RunScan(db, inputV1)
	require.NoError(t, err)
	srV2, err := RunScan(db, inputV2)
	require.NoError(t, err)

	var gotV1, gotV2 tradingstack.ScanResult
	require.NoError(t, db.First(&gotV1, "id = ?", srV1.ID).Error)
	require.NoError(t, db.First(&gotV2, "id = ?", srV2.ID).Error)

	require.NotNil(t, gotV1.ScannerVersion)
	require.NotNil(t, gotV2.ScannerVersion)
	assert.Equal(t, "l1l2-v1", *gotV1.ScannerVersion)
	assert.Equal(t, "l1l2-v2", *gotV2.ScannerVersion)
}
