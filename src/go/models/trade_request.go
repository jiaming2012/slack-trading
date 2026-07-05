package models

import (
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
)

// Validate is ported from the pre-merge legacy models.CloseTradeRequestV1 onto
// the canonical (merged) CloseTradeRequestV1 type, which has identical fields
// (reconcile-models-packages: method port).
func (r *CloseTradeRequestV1) Validate() error {
	if r.Reason == "" {
		return fmt.Errorf("CloseTradeRequestV1: reason was not set")
	}

	if math.Abs(r.Volume) == 0 {
		return TradeVolumeIsZeroErr
	}

	if r.Timeframe <= 0 {
		return InvalidTimeframeErr
	}

	if r.Trade == nil {
		return fmt.Errorf("CloseTradeRequestV1: closing trade not set")
	}

	if math.Abs(r.Trade.ExecutedVolume) == 0 {
		return fmt.Errorf("CloseTradeRequestV1: closing trade executed volume is zero")
	}

	if math.Abs(r.Volume) > math.Abs(r.Trade.ExecutedVolume) {
		return fmt.Errorf("CloseTradeRequestV1: volume of close request cannot exceed trade volume")
	}

	return nil
}

// todo: remove legacy models
type BulkCloseRequestItem struct {
	Level        *PriceLevel
	ClosePercent float64
}

// todo: remove legacy models
type BulkCloseRequest struct {
	Items []BulkCloseRequestItem
}

// todo: remove legacy models
func (r *BulkCloseRequest) Execute(price float64, symbol string, timeframe *int) ([]*Trade, error) {
	trades := make([]*Trade, 0)
	for _, it := range r.Items {
		if it.ClosePercent < 0 || it.ClosePercent > 1 {
			return nil, InvalidClosePercentErr
		}

		if it.Level.Trades != nil {
			_, vol, _ := it.Level.Trades.GetTradeStatsItems()
			closeVol := float64(vol) * it.ClosePercent * -1
			tr, _, err := NewOpenTrade(uuid.New(), TradeTypeClose, symbol, timeframe, time.Now().UTC(), price, closeVol, 0, nil)
			if err != nil {
				return nil, fmt.Errorf("BulkCloseRequest.Execute: failed to open NewTrade: %w", err)
			}

			tr.RequestedVolume = closeVol

			it.Level.Trades.Add(tr)
			trades = append(trades, tr)
		}
	}

	return trades, nil
}

type OpenTradeRequest struct {
	Timeframe *int
	Strategy  *Strategy
}

func (r *OpenTradeRequest) Validate() error {
	if r.Strategy == nil {
		return fmt.Errorf("OpenTradeRequest.Validate: strategy not set")
	}

	if r.Timeframe != nil && *r.Timeframe <= 0 {
		return InvalidTimeframeErr
	}

	return nil
}
