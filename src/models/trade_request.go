package models

import (
	"fmt"
	"github.com/google/uuid"
	"math"
	"time"
)

type CloseTradesRequestV1 []*CloseTradeRequestV1

type CloseTradeRequestV1 struct {
	Trade     *Trade
	Strategy  *Strategy
	Timeframe int
	Volume    float64
	Reason    string
}

type CloseTradeRequestV2 struct {
	Trade     *Trade
	Timeframe *int
	Percent   float64
	Reason    string
}

func (r *CloseTradeRequestV2) Validate() error {
	if r.Trade == nil {
		return fmt.Errorf("CloseTradesRequest.Validate: trade not set")
	}

	if r.Timeframe != nil && *r.Timeframe <= 0 {
		return InvalidTimeframeErr
	}

	if r.Percent < 0 || r.Percent > 1 {
		return InvalidClosePercentErr
	}

	return nil
}

type CloseTradesRequest struct {
	Strategy        *Strategy
	Timeframe       *int
	PriceLevelIndex int
	Percent         float64
	Reason          string
}

func (r *CloseTradesRequest) Validate() error {
	if r.Strategy == nil {
		return fmt.Errorf("CloseTradesRequest.Validate: strategy not set")
	}

	if r.Timeframe != nil && *r.Timeframe <= 0 {
		return InvalidTimeframeErr
	}

	if r.PriceLevelIndex < 0 {
		return fmt.Errorf("CloseTradesRequest.Validate: found %v: %w", r.PriceLevelIndex, InvalidPriceLevelIndexErr)
	}

	if r.Percent < 0 || r.Percent > 1 {
		return InvalidClosePercentErr
	}

	if r.Reason == "" {
		return fmt.Errorf("CloseTradesRequest.Validate: reason not set")
	}

	return nil
}

func NewCloseTradesRequest(strategy *Strategy, timeframe *int, priceLevelIndex int, percent float64, reason string) (*CloseTradesRequest, error) {
	closeReq := &CloseTradesRequest{Strategy: strategy, Timeframe: timeframe, PriceLevelIndex: priceLevelIndex, Percent: percent, Reason: reason}

	if err := closeReq.Validate(); err != nil {
		return nil, fmt.Errorf("NewCloseTradesRequest validation failed: %w", err)
	}

	return closeReq, nil
}

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
			tr, _, err := NewOpenTrade(uuid.New(), TradeTypeClose, symbol, timeframe, time.Now(), price, closeVol, 0, nil)
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

func NewOpenTradeRequest(timeframe *int, strategy *Strategy) (*OpenTradeRequest, error) {
	request := &OpenTradeRequest{
		Timeframe: timeframe,
		Strategy:  strategy,
	}

	if err := request.Validate(); err != nil {
		return nil, fmt.Errorf("NewOpenTradeRequest validation failed: %w", err)
	}

	return request, nil
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
