package data

// Round-trip tests for the persisted deferred-auto-close store
// (wire-companion-stops, adversarial-review finding 2), against a disposable
// Postgres container: save stamps the record ID, load reproduces the close
// request faithfully (including the fill parameters a retried commit relies
// on), delete removes the row for good, and playgrounds are isolated.

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	backtester_models "github.com/jiaming2012/slack-trading/src/go/backtester/models"
)

func TestDeferredAutoCloseStore_RoundTrip(t *testing.T) {
	db := newTestDB(t)
	svc := NewDatabaseService(db, nil, nil)

	playgroundID := uuid.New()
	otherPlaygroundID := uuid.New()
	closeOrderID := uint(7)
	fillTime := time.Date(2025, time.July, 3, 16, 0, 0, 0, time.UTC)

	deferral := &backtester_models.DeferredAutoClose{
		Request: &backtester_models.CreateOrderRequest{
			Symbol:         "O:AAPL250703C00210000",
			Class:          backtester_models.OrderRecordClassOption,
			Quantity:       5,
			Side:           backtester_models.TradierOrderSideBuyToClose,
			OrderType:      backtester_models.Market,
			Duration:       backtester_models.Day,
			RequestedPrice: 5.0,
			Tag:            "auto-closed-on-early-assignment",
			CloseOrderId:   &closeOrderID,
			IsSystemOrder:  true,
		},
		SourceOrderID:       7,
		FillTime:            fillTime,
		EmitAssignmentEvent: true,
		Reason:              "order submission halted by kill switch: drill",
	}

	// Save stamps the record ID onto the in-memory deferral.
	require.NoError(t, svc.SaveDeferredAutoClose(playgroundID, deferral))
	require.NotZero(t, deferral.RecordID)

	// A second playground's deferral must not leak into the first's load.
	other := &backtester_models.DeferredAutoClose{
		Request:       &backtester_models.CreateOrderRequest{Symbol: "AAPL", Class: backtester_models.OrderRecordClassEquity, Quantity: 1, Side: backtester_models.TradierOrderSideSell, OrderType: backtester_models.Market, Duration: backtester_models.Day},
		SourceOrderID: 9,
		FillTime:      fillTime,
	}
	require.NoError(t, svc.SaveDeferredAutoClose(otherPlaygroundID, other))

	loaded, err := svc.LoadDeferredAutoCloses(playgroundID)
	require.NoError(t, err)
	require.Len(t, loaded, 1)

	got := loaded[0]
	require.Equal(t, deferral.RecordID, got.RecordID)
	require.Equal(t, uint(7), got.SourceOrderID)
	require.True(t, got.EmitAssignmentEvent)
	require.Contains(t, got.Reason, "halted by kill switch")
	require.WithinDuration(t, fillTime, got.FillTime, time.Millisecond, "the retained fill time must survive the round trip")

	// The close request — what the retried commit places — round-trips
	// faithfully.
	require.NotNil(t, got.Request)
	require.Equal(t, "O:AAPL250703C00210000", got.Request.Symbol)
	require.Equal(t, backtester_models.OrderRecordClassOption, got.Request.Class)
	require.Equal(t, 5.0, got.Request.Quantity)
	require.Equal(t, backtester_models.TradierOrderSideBuyToClose, got.Request.Side)
	require.Equal(t, 5.0, got.Request.RequestedPrice)
	require.Equal(t, "auto-closed-on-early-assignment", got.Request.Tag)
	require.NotNil(t, got.Request.CloseOrderId)
	require.Equal(t, closeOrderID, *got.Request.CloseOrderId)
	require.True(t, got.Request.IsSystemOrder)

	// Delete removes the row for good (hard delete — a soft-deleted row must
	// never resurrect a committed close on restart).
	require.NoError(t, svc.DeleteDeferredAutoClose(deferral.RecordID))
	loaded, err = svc.LoadDeferredAutoCloses(playgroundID)
	require.NoError(t, err)
	require.Empty(t, loaded)

	var rowCount int64
	require.NoError(t, db.Unscoped().Model(&backtester_models.DeferredAutoCloseRecord{}).
		Where("playground_id = ?", playgroundID).Count(&rowCount).Error)
	require.Zero(t, rowCount, "the delete must be a hard delete")

	// Deleting an unpersisted (zero-ID) deferral is a no-op.
	require.NoError(t, svc.DeleteDeferredAutoClose(0))

	// The other playground's deferral is untouched.
	otherLoaded, err := svc.LoadDeferredAutoCloses(otherPlaygroundID)
	require.NoError(t, err)
	require.Len(t, otherLoaded, 1)
}
