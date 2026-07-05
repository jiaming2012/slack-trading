package router

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/jiaming2012/slack-trading/src/go/backtester-api/models"
	eventmodels "github.com/jiaming2012/slack-trading/src/go/models"
	pb "github.com/jiaming2012/slack-trading/src/go/playground"
)

func newSignalTestServer() *Server {
	return &Server{
		cache:            models.NewRequestCache(),
		globalSignalRepo: models.NewInMemorySignalRepository(),
	}
}

func TestWriteSignal_HappyPath(t *testing.T) {
	s := newSignalTestServer()
	ctx := context.Background()

	resp, err := s.WriteSignal(ctx, &pb.WriteSignalRequest{
		Name:       "ma_crossover",
		Symbol:     "AAPL",
		Timestamp:  timestamppb.Now(),
		Attributes: map[string]string{"direction": "up"},
	})

	require.NoError(t, err)
	require.NotEmpty(t, resp.SignalId)

	// Validate it's a valid UUID
	_, parseErr := uuid.Parse(resp.SignalId)
	assert.NoError(t, parseErr)

	// Validate signal was stored
	all := s.globalSignalRepo.GetAll()
	require.Len(t, all, 1)
	assert.Equal(t, eventmodels.SignalName("ma_crossover"), all[0].Name)
	assert.Equal(t, eventmodels.StockSymbol("AAPL"), all[0].Symbol)
}

func TestWriteSignal_InvalidName(t *testing.T) {
	s := newSignalTestServer()
	ctx := context.Background()

	_, err := s.WriteSignal(ctx, &pb.WriteSignalRequest{
		Name:      "invalid_signal_xyz",
		Symbol:    "AAPL",
		Timestamp: timestamppb.Now(),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid signal name")
}

func TestWriteSignal_MissingSymbol(t *testing.T) {
	s := newSignalTestServer()
	ctx := context.Background()

	_, err := s.WriteSignal(ctx, &pb.WriteSignalRequest{
		Name:      "ma_crossover",
		Symbol:    "",
		Timestamp: timestamppb.Now(),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "symbol is required")
}

func TestWriteSignal_MissingTimestamp(t *testing.T) {
	s := newSignalTestServer()
	ctx := context.Background()

	_, err := s.WriteSignal(ctx, &pb.WriteSignalRequest{
		Name:   "ma_crossover",
		Symbol: "AAPL",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "timestamp is required")
}

func TestGetSignals_AllSignals(t *testing.T) {
	s := newSignalTestServer()
	ctx := context.Background()
	now := time.Now()

	// Write 3 signals: 2 AAPL ma_crossover, 1 MSFT start_of_week
	require.NoError(t, s.globalSignalRepo.Write(eventmodels.NewTradeSignal("ma_crossover", "AAPL", now, nil)))
	require.NoError(t, s.globalSignalRepo.Write(eventmodels.NewTradeSignal("ma_crossover", "AAPL", now.Add(time.Minute), nil)))
	require.NoError(t, s.globalSignalRepo.Write(eventmodels.NewTradeSignal("start_of_week", "MSFT", now.Add(2*time.Minute), nil)))

	resp, err := s.GetSignals(ctx, &pb.GetSignalsRequest{})
	require.NoError(t, err)
	assert.Len(t, resp.Signals, 3)
}

func TestGetSignals_FilterByName(t *testing.T) {
	s := newSignalTestServer()
	ctx := context.Background()
	now := time.Now()

	require.NoError(t, s.globalSignalRepo.Write(eventmodels.NewTradeSignal("ma_crossover", "AAPL", now, nil)))
	require.NoError(t, s.globalSignalRepo.Write(eventmodels.NewTradeSignal("ma_crossover", "AAPL", now.Add(time.Minute), nil)))
	require.NoError(t, s.globalSignalRepo.Write(eventmodels.NewTradeSignal("start_of_week", "MSFT", now.Add(2*time.Minute), nil)))

	nameFilter := "ma_crossover"
	resp, err := s.GetSignals(ctx, &pb.GetSignalsRequest{
		Name: &nameFilter,
	})
	require.NoError(t, err)
	assert.Len(t, resp.Signals, 2)
	for _, sig := range resp.Signals {
		assert.Equal(t, "ma_crossover", sig.Name)
	}
}

func TestGetSignals_FilterBySymbol(t *testing.T) {
	s := newSignalTestServer()
	ctx := context.Background()
	now := time.Now()

	require.NoError(t, s.globalSignalRepo.Write(eventmodels.NewTradeSignal("ma_crossover", "AAPL", now, nil)))
	require.NoError(t, s.globalSignalRepo.Write(eventmodels.NewTradeSignal("ma_crossover", "AAPL", now.Add(time.Minute), nil)))
	require.NoError(t, s.globalSignalRepo.Write(eventmodels.NewTradeSignal("start_of_week", "MSFT", now.Add(2*time.Minute), nil)))

	symbolFilter := "MSFT"
	resp, err := s.GetSignals(ctx, &pb.GetSignalsRequest{
		Symbol: &symbolFilter,
	})
	require.NoError(t, err)
	require.Len(t, resp.Signals, 1)
	assert.Equal(t, "MSFT", resp.Signals[0].Symbol)
}

func TestGetSignals_FilterByTimeRange(t *testing.T) {
	s := newSignalTestServer()
	ctx := context.Background()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	require.NoError(t, s.globalSignalRepo.Write(eventmodels.NewTradeSignal("ma_crossover", "AAPL", base, nil)))
	require.NoError(t, s.globalSignalRepo.Write(eventmodels.NewTradeSignal("ma_crossover", "AAPL", base.Add(time.Hour), nil)))
	require.NoError(t, s.globalSignalRepo.Write(eventmodels.NewTradeSignal("ma_crossover", "AAPL", base.Add(2*time.Hour), nil)))

	startTime := timestamppb.New(base.Add(30 * time.Minute))
	endTime := timestamppb.New(base.Add(90 * time.Minute))

	resp, err := s.GetSignals(ctx, &pb.GetSignalsRequest{
		StartTime: startTime,
		EndTime:   endTime,
	})
	require.NoError(t, err)
	require.Len(t, resp.Signals, 1)
	assert.Equal(t, base.Add(time.Hour).Unix(), resp.Signals[0].Timestamp.AsTime().Unix())
}

func TestGetProcessedSignals_MissingPlaygroundId(t *testing.T) {
	s := newSignalTestServer()
	ctx := context.Background()

	_, err := s.GetProcessedSignals(ctx, &pb.GetProcessedSignalsRequest{
		PlaygroundId: "",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "playground_id is required")
}

func TestGetProcessedSignals_InvalidPlaygroundId(t *testing.T) {
	s := newSignalTestServer()
	ctx := context.Background()

	_, err := s.GetProcessedSignals(ctx, &pb.GetProcessedSignalsRequest{
		PlaygroundId: "not-a-uuid",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid playground_id")
}

// TODO: integration test for GetProcessedSignals happy path with real DatabaseService
