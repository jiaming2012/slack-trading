package data

// Tests for halt-rejection hygiene (wire-companion-stops, review nit d): when a
// live-Mode (non-Simulation) order is rejected at the halt-gated submission
// path AFTER its database row was pre-created by commitOrderRecord, that row
// must be finalized as rejected with the reason recorded — never left as an
// orphan pending row. The same finalization applies to any placement failure
// after row pre-creation. Simulation pre-creates no row and is byte-for-byte
// unchanged. Everything runs against MockBroker + a disposable Postgres
// container; no real broker or production database is touched.

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	backtester_models "github.com/jiaming2012/slack-trading/src/go/backtester/models"
	"github.com/jiaming2012/slack-trading/src/go/models"
)

// newTestDB boots a disposable postgres:13 container and migrates the order
// tables — same harness pattern as the telemetry and tradingstack suites.
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	ctx := context.Background()

	const (
		user   = "test"
		pass   = "test"
		dbName = "orders_test"
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

	require.NoError(t, db.AutoMigrate(&backtester_models.OrderRecord{}, &backtester_models.DeferredAutoCloseRecord{}))

	return db
}

// stubOrderGate is a fixed-answer order gate: nil-error permits, non-nil
// rejects with that error (the halt rejection).
type stubOrderGate struct{ err error }

func (g stubOrderGate) AllowOrder() error { return g.err }

// haltFixture wires a Paper-Mode playground into a REAL DatabaseService backed
// by the container database, with the Broker seam bound to MockBroker. The
// playground's reconcile pointer is deliberately wired to itself: the
// "cannot place order in the same playground" check then provides a
// deterministic non-halt placement failure for the finalization tests, while
// the halt tests reject at the gate before the reconcile pointer is consulted.
type haltFixture struct {
	svc            *DatabaseService
	livePlayground *backtester_models.Playground
	liveID         uuid.UUID
}

func newHaltFixture(t *testing.T, db *gorm.DB) *haltFixture {
	t.Helper()

	startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
	symbol := models.NewStockSymbol("AAPL")

	feed := []*models.PolygonAggregateBarV2{
		{Timestamp: startTime, Close: 100.0},
		{Timestamp: startTime.Add(time.Minute), Close: 101.0},
	}

	svc := NewDatabaseService(db, nil, nil)
	broker := backtester_models.NewMockBroker(1000, nil)
	liveAccount, err := backtester_models.NewLiveAccount(broker, svc)
	require.NoError(t, err)

	repo, err := backtester_models.NewCandleRepository(symbol, time.Minute, feed, []string{}, nil, 0, models.CandleRepositorySource{Type: "test"})
	require.NoError(t, err)

	liveID := uuid.New()
	clientID := "halt-finalize-test"
	accountRequestSource := backtester_models.NewMockLiveAccountSource()
	source := &backtester_models.CreateAccountRequestSource{
		Broker:      accountRequestSource.GetBroker(),
		AccountID:   accountRequestSource.GetAccountID(),
		AccountRole: accountRequestSource.GetAccountType(),
	}

	newTradesQueue := models.NewFIFOQueue[*backtester_models.TradeRecord]("newTradesFilledQueue", 4)

	// Self-wire the reconcile pointer (see type comment).
	livePlayground := &backtester_models.Playground{}
	selfReconcile, err := backtester_models.NewReconcilePlayground(livePlayground, liveAccount)
	require.NoError(t, err)

	require.NoError(t, backtester_models.PopulatePlayground(livePlayground, &backtester_models.PopulatePlaygroundRequest{
		ID:                  &liveID,
		ClientID:            &clientID,
		Mode:                backtester_models.ModePaper,
		Account:             backtester_models.CreateAccountRequest{Balance: 100000.0, Source: source},
		InitialBalance:      100000.0,
		BackfillOrders:      []*backtester_models.OrderRecord{},
		Tags:                []string{},
		LiveAccount:         liveAccount,
		ReconcilePlayground: selfReconcile,
	}, nil, startTime, newTradesQueue, nil, nil, repo))

	require.NoError(t, svc.SavePlaygroundInMemory(livePlayground))

	return &haltFixture{svc: svc, livePlayground: livePlayground, liveID: liveID}
}

func buyRequest(qty float64) *backtester_models.CreateOrderRequest {
	return &backtester_models.CreateOrderRequest{
		Symbol:         "AAPL",
		Class:          backtester_models.OrderRecordClassEquity,
		Quantity:       qty,
		Side:           backtester_models.TradierOrderSideBuy,
		OrderType:      backtester_models.Market,
		Duration:       backtester_models.Day,
		RequestedPrice: 100.0,
	}
}

func fetchAllOrderRows(t *testing.T, db *gorm.DB) []backtester_models.OrderRecord {
	t.Helper()
	var rows []backtester_models.OrderRecord
	require.NoError(t, db.Order("id asc").Find(&rows).Error)
	return rows
}

// An engaged halt rejecting a Paper-Mode order must leave its pre-created row
// finalized as rejected with the halt reason — never pending.
func TestHaltRejectedLiveOrderRowFinalizedAsRejected(t *testing.T) {
	db := newTestDB(t)
	f := newHaltFixture(t, db)

	haltErr := fmt.Errorf("order submission halted by kill switch (source=manual): drill")
	backtester_models.SetOrderGate(stubOrderGate{err: haltErr})
	t.Cleanup(func() { backtester_models.SetOrderGate(nil) })

	_, err := f.svc.PlaceOrders(f.liveID, []*backtester_models.CreateOrderRequest{buyRequest(10)})
	require.Error(t, err)
	require.Contains(t, err.Error(), "halted by kill switch")

	rows := fetchAllOrderRows(t, db)
	require.Len(t, rows, 1, "the pre-created row must exist (marked, not deleted — audit trail)")

	row := rows[0]
	require.Equal(t, backtester_models.OrderRecordStatusRejected, row.Status, "the halt-rejected row must be finalized as rejected, never pending")
	require.NotNil(t, row.RejectReason)
	require.Contains(t, *row.RejectReason, "halted by kill switch", "the rejection reason must record the halt")
	require.Equal(t, "AAPL", row.Symbol, "the finalized row must carry the attempted order's fields for the audit trail")

	var pendingCount int64
	require.NoError(t, db.Model(&backtester_models.OrderRecord{}).
		Where("status = ?", backtester_models.OrderRecordStatusPending).Count(&pendingCount).Error)
	require.Zero(t, pendingCount, "a halt rejection must leave no orphan pending rows")
}

// Any non-halt Playground.PlaceOrder failure after the row was pre-created must
// finalize it the same way.
func TestNonHaltPlacementFailureAlsoFinalizesRow(t *testing.T) {
	db := newTestDB(t)
	f := newHaltFixture(t, db)

	// No gate installed: placement proceeds past the halt gate and fails on the
	// fixture's deliberate self-reconcile wiring.
	_, err := f.svc.PlaceOrders(f.liveID, []*backtester_models.CreateOrderRequest{buyRequest(5)})
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot place order in the same playground")

	rows := fetchAllOrderRows(t, db)
	require.Len(t, rows, 1)
	require.Equal(t, backtester_models.OrderRecordStatusRejected, rows[0].Status)
	require.NotNil(t, rows[0].RejectReason)
	require.Contains(t, *rows[0].RejectReason, "cannot place order in the same playground")
}

// Simulation pre-creates no database row, so a halt rejection there must leave
// the database untouched — the Simulation path is byte-for-byte unchanged.
func TestSimulationHaltRejectionCreatesNoRow(t *testing.T) {
	db := newTestDB(t)
	svc := NewDatabaseService(db, nil, nil)

	startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2021, time.January, 2, 0, 0, 0, 0, time.UTC)
	symbol := models.NewStockSymbol("AAPL")
	feed := []*models.PolygonAggregateBarV2{
		{Timestamp: startTime, Close: 100.0},
		{Timestamp: startTime.Add(time.Minute), Close: 101.0},
	}

	repo, err := backtester_models.NewCandleRepository(symbol, time.Minute, feed, []string{}, nil, 0, models.CandleRepositorySource{Type: "test"})
	require.NoError(t, err)

	playground, err := backtester_models.NewPlayground(backtester_models.PlaygroundConfig{
		Balance: 100000.0,
		Clock:   backtester_models.NewClock(startTime, endTime, nil),
		Mode:    backtester_models.ModeSimulation,
		Now:     startTime,
		Feeds:   []*backtester_models.CandleRepository{repo},
	})
	require.NoError(t, err)
	require.NoError(t, svc.SavePlaygroundInMemory(playground))

	backtester_models.SetOrderGate(stubOrderGate{err: fmt.Errorf("order submission halted by kill switch")})
	t.Cleanup(func() { backtester_models.SetOrderGate(nil) })

	_, err = svc.PlaceOrders(playground.GetId(), []*backtester_models.CreateOrderRequest{buyRequest(10)})
	require.Error(t, err)
	require.Contains(t, err.Error(), "halted by kill switch")

	rows := fetchAllOrderRows(t, db)
	require.Empty(t, rows, "Simulation pre-creates no row, so there is nothing to finalize")
}
