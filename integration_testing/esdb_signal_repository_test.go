//go:build integration

package integrationtesting

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/EventStore/EventStore-Client-Go/v4/esdb"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/jiaming2012/slack-trading/src/go/backtester-api/models"
	eventmodels "github.com/jiaming2012/slack-trading/src/go/models"
	"github.com/jiaming2012/slack-trading/src/go/eventproducers"
	"github.com/jiaming2012/slack-trading/src/go/eventpubsub"
	"github.com/jiaming2012/slack-trading/src/go/eventservices"
)

// startESDBContainer spins up an EventStoreDB 24.2.0 container via TestContainers
// and returns the connection string for the esdb client.
func startESDBContainer(t *testing.T, ctx context.Context) string {
	t.Helper()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "eventstore/eventstore:24.2.0",
			ExposedPorts: []string{"2113/tcp"},
			Env: map[string]string{
				"EVENTSTORE_INSECURE":                  "true",
				"EVENTSTORE_CLUSTER_SIZE":              "1",
				"EVENTSTORE_RUN_PROJECTIONS":           "All",
				"EVENTSTORE_START_STANDARD_PROJECTIONS": "false",
				"EVENTSTORE_MEM_DB":                    "true",
			},
			WaitingFor: wait.ForHTTP("/health/live").
				WithPort("2113/tcp").
				WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	require.NoError(t, err)
	testcontainers.CleanupContainer(t, container)

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "2113/tcp")
	require.NoError(t, err)

	return "esdb://admin:changeit@" + host + ":" + port.Port() + "?tls=false"
}

// createESDBProducer initializes pub/sub, creates an EsdbProducer, and calls Start.
// Returns the producer and a cancel function to clean up.
func createESDBProducer(t *testing.T, connectionString string) (*eventproducers.EsdbProducer, context.CancelFunc) {
	t.Helper()

	eventpubsub.Init()

	var wg sync.WaitGroup
	producer := eventproducers.NewESDBProducer(&wg, connectionString, nil)

	ctx, cancel := context.WithCancel(context.Background())
	fxTicksCh := make(chan *eventmodels.FxTick)
	producer.Start(ctx, fxTicksCh)

	return producer, cancel
}

func TestESDBSignalRepository_WriteReadRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("requires docker")
	}

	ctx := context.Background()

	// Start ESDB container
	connStr := startESDBContainer(t, ctx)

	// Create producer and repository
	producer, cancel := createESDBProducer(t, connStr)
	defer cancel()

	repo := models.NewESDBSignalRepository(producer)

	// Define test timestamps
	t1 := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2025, 1, 15, 11, 0, 0, 0, time.UTC)
	t3 := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

	// Write 3 signals with different names/symbols/timestamps
	signal1 := eventmodels.NewTradeSignal(eventmodels.SignalMACrossover, "AAPL", t1, map[string]interface{}{"direction": "up"})
	signal2 := eventmodels.NewTradeSignal(eventmodels.SignalMeanReversion, "MSFT", t2, map[string]interface{}{"zscore": "2.1"})
	signal3 := eventmodels.NewTradeSignal(eventmodels.SignalMACrossover, "AAPL", t3, map[string]interface{}{"direction": "down"})

	require.NoError(t, repo.Write(signal1))
	require.NoError(t, repo.Write(signal2))
	require.NoError(t, repo.Write(signal3))

	// Test 1: Read all signals back, verify count=3
	allSignals := repo.GetAll()
	require.Len(t, allSignals, 3, "expected 3 signals after writing 3")

	// Test 2: ReadPending with cutoff time returns only signals at or before that time
	// Using t2 + 30 minutes as cutoff should return signals at t1 and t2
	cutoff := t2.Add(30 * time.Minute)
	pending := repo.ReadPending(cutoff)
	assert.Len(t, pending, 2, "expected 2 signals at or before cutoff")

	// Test 3: Filter by name -- verify only matching signals returned
	var macrossoverSignals []*eventmodels.TradeSignal
	for _, s := range allSignals {
		if s.Name == eventmodels.SignalMACrossover {
			macrossoverSignals = append(macrossoverSignals, s)
		}
	}
	assert.Len(t, macrossoverSignals, 2, "expected 2 MA_CROSSOVER signals")

	var meanRevSignals []*eventmodels.TradeSignal
	for _, s := range allSignals {
		if s.Name == eventmodels.SignalMeanReversion {
			meanRevSignals = append(meanRevSignals, s)
		}
	}
	assert.Len(t, meanRevSignals, 1, "expected 1 MEAN_REVERSION signal")

	// Test 4: Filter by symbol -- verify only matching signals returned
	var aaplSignals []*eventmodels.TradeSignal
	for _, s := range allSignals {
		if s.Symbol == "AAPL" {
			aaplSignals = append(aaplSignals, s)
		}
	}
	assert.Len(t, aaplSignals, 2, "expected 2 AAPL signals")

	var msftSignals []*eventmodels.TradeSignal
	for _, s := range allSignals {
		if s.Symbol == "MSFT" {
			msftSignals = append(msftSignals, s)
		}
	}
	assert.Len(t, msftSignals, 1, "expected 1 MSFT signal")
}

func TestESDBSignalRepository_FetchAllDirect(t *testing.T) {
	if testing.Short() {
		t.Skip("requires docker")
	}

	ctx := context.Background()

	// Start ESDB container
	connStr := startESDBContainer(t, ctx)

	// Create an ESDB client directly for writing
	settings, err := esdb.ParseConnectionString(connStr)
	require.NoError(t, err)

	client, err := esdb.NewClient(settings)
	require.NoError(t, err)
	defer client.Close()

	// Write signals directly via AppendToStream
	t1 := time.Date(2025, 2, 1, 9, 30, 0, 0, time.UTC)
	t2 := time.Date(2025, 2, 1, 10, 0, 0, 0, time.UTC)

	signals := []*eventmodels.TradeSignal{
		eventmodels.NewTradeSignal(eventmodels.SignalCoveredCall, "SPY", t1, nil),
		eventmodels.NewTradeSignal(eventmodels.SignalStartOfWeek, "QQQ", t2, nil),
	}

	streamName := string(eventmodels.TradeSignalStream)
	for _, sig := range signals {
		data, err := json.Marshal(sig)
		require.NoError(t, err)

		eventData := esdb.EventData{
			ContentType: esdb.ContentTypeJson,
			EventType:   string(eventmodels.TradeSignalEventName),
			Data:        data,
			EventID:     uuid.New(),
		}

		_, err = client.AppendToStream(ctx, streamName, esdb.AppendToStreamOptions{}, eventData)
		require.NoError(t, err)
	}

	// Read signals via FetchAll (same path as ESDBSignalRepository.fetchAllFromESDB)
	result, err := eventservices.FetchAll[*eventmodels.TradeSignal](ctx, client, &eventmodels.TradeSignal{})
	require.NoError(t, err)
	require.Len(t, result, 2, "expected 2 signals from FetchAll")

	// Verify data integrity
	assert.Equal(t, eventmodels.SignalCoveredCall, result[0].Name)
	assert.Equal(t, eventmodels.StockSymbol("SPY"), result[0].Symbol)
	assert.True(t, result[0].Timestamp.Equal(t1))

	assert.Equal(t, eventmodels.SignalStartOfWeek, result[1].Name)
	assert.Equal(t, eventmodels.StockSymbol("QQQ"), result[1].Symbol)
	assert.True(t, result[1].Timestamp.Equal(t2))
}
