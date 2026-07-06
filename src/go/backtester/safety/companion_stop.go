package safety

import (
	"context"
	"fmt"

	models "github.com/jiaming2012/slack-trading/src/go/backtester/models"
)

// CompanionStopConfig configures the broker-held companion stop placed on a live
// entry fill. StopDistance is the absolute price distance from the fill price at
// which the protective stop sits. It must be positive; a non-positive distance
// is rejected rather than placing a stop at or through the fill price.
type CompanionStopConfig struct {
	StopDistance float64
}

// Validate enforces a positive stop distance.
func (c CompanionStopConfig) Validate() error {
	if c.StopDistance <= 0 {
		return fmt.Errorf("companion stop distance must be positive, got %v", c.StopDistance)
	}
	return nil
}

// EntryFill describes a filled live entry order for which a protective
// companion stop must be placed.
type EntryFill struct {
	Symbol    string
	EntrySide models.TradierOrderSide
	Quantity  int
	FillPrice float64

	// EntryOrderID, when non-zero, is the entry order this stop protects. It is
	// encoded into the stop order's tag (see CompanionStopTagForEntry) — the
	// durable association that makes placement idempotent per entry order
	// across event redelivery and process restarts.
	EntryOrderID uint
}

// PlaceCompanionStop places a broker-held stop order that protects a just-filled
// live entry, so the exit executes at the Broker even if our own infrastructure
// is completely down.
//
//   - Simulation Mode places nothing (exits are handled by the simulated fill
//     engine); it returns (nil, nil).
//   - Paper and Margin Modes place a stop on the protective side of the position
//     at StopDistance from the fill price: below the fill for a long, above the
//     fill for a short, sized to the filled quantity.
//   - A non-positive stop distance (or a distance that would place the stop at or
//     through zero) returns an error and places nothing.
//
// The stop is placed through the IBroker seam; overnight this is exercised
// against MockBroker only (Tradier sandbox verification is deferred). The placed
// request is returned so callers and tests can assert its side, quantity, and
// stop price.
func PlaceCompanionStop(ctx context.Context, broker models.IBroker, mode models.Mode, fill EntryFill, cfg CompanionStopConfig) (*models.PlaceOrderRequest, error) {
	if mode == models.ModeSimulation {
		return nil, nil
	}

	if !mode.IsRealtime() {
		return nil, fmt.Errorf("PlaceCompanionStop: companion stops apply only to live modes, got %q", mode)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("PlaceCompanionStop: %w", err)
	}

	if broker == nil {
		return nil, fmt.Errorf("PlaceCompanionStop: broker is nil")
	}

	if fill.Quantity <= 0 {
		return nil, fmt.Errorf("PlaceCompanionStop: fill quantity must be positive, got %d", fill.Quantity)
	}

	if fill.FillPrice <= 0 {
		return nil, fmt.Errorf("PlaceCompanionStop: fill price must be positive, got %v", fill.FillPrice)
	}

	protectiveSide, stopPrice, err := companionStopParams(fill.EntrySide, fill.FillPrice, cfg.StopDistance)
	if err != nil {
		return nil, fmt.Errorf("PlaceCompanionStop: %w", err)
	}

	tag := CompanionStopTag
	if fill.EntryOrderID > 0 {
		tag = CompanionStopTagForEntry(fill.EntryOrderID)
	}

	req := &models.PlaceOrderRequest{
		Symbol:     fill.Symbol,
		Quantities: []int{fill.Quantity},
		Sides:      []models.TradierOrderSide{protectiveSide},
		OrderType:  models.TradierOrderTypeStop,
		Class:      models.OrderRecordClassEquity,
		Tag:        tag,
		StopPrice:  &stopPrice,
	}

	if _, err := broker.PlaceOrder(ctx, req); err != nil {
		return nil, fmt.Errorf("PlaceCompanionStop: place stop order: %w", err)
	}

	return req, nil
}

// companionStopParams computes the protective side and stop trigger price for an
// equity entry. A long entry (buy) is protected by a sell stop below the fill; a
// short entry (sell_short) is protected by a buy_to_cover stop above the fill.
func companionStopParams(entrySide models.TradierOrderSide, fillPrice, distance float64) (models.TradierOrderSide, float64, error) {
	switch entrySide {
	case models.TradierOrderSideBuy:
		stop := fillPrice - distance
		if stop <= 0 {
			return "", 0, fmt.Errorf("long stop distance %v places the stop at or below zero (fill %v)", distance, fillPrice)
		}
		return models.TradierOrderSideSell, stop, nil
	case models.TradierOrderSideSellShort:
		stop := fillPrice + distance
		return models.TradierOrderSideBuyToCover, stop, nil
	default:
		return "", 0, fmt.Errorf("unsupported entry side for companion stop (equity buy or sell_short only): %s", entrySide)
	}
}
