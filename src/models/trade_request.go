package models

import (
	"fmt"
	"github.com/google/uuid"
	"math"
	"time"
)

type CloseTradesRequest []*CloseTradeRequestV1

type CloseTradeRequestV1 struct {
	Trade     *Trade
	Strategy  *Strategy
	Timeframe int
	Volume    float64
	Reason    string
}

type CloseTradesRequestV2 struct {
	Strategy        *Strategy
	Timeframe       int
	PriceLevelIndex int
	Percent         float64
}

func (r *CloseTradesRequestV2) Validate() error {
	if r.Strategy == nil {
		return fmt.Errorf("CloseTradesRequestV2.Validate: strategy not set")
	}

	if r.Timeframe <= 0 {
		return InvalidTimeframeErr
	}

	if r.PriceLevelIndex <= 0 {
		return fmt.Errorf("CloseTradesRequestV2.Validate: found %v: %w", r.PriceLevelIndex, InvalidPriceLevelIndexErr)
	}

	if r.Percent < 0 || r.Percent > 1 {
		return InvalidClosePercentErr
	}

	return nil
}

func NewCloseTradesRequestV2(strategy *Strategy, timeframe int, priceLevelIndex int, percent float64) (*CloseTradesRequestV2, error) {
	closeReq := &CloseTradesRequestV2{Strategy: strategy, Timeframe: timeframe, PriceLevelIndex: priceLevelIndex, Percent: percent}

	if err := closeReq.Validate(); err != nil {
		return nil, fmt.Errorf("NewCloseTradesRequestV2 validation failed: %w", err)
	}

	return closeReq, nil
}

func NewCloseTradesRequestV1(id uuid.UUID, timeframe int, timestamp time.Time, requestedPrice float64, reason string, trades Trades) (CloseTradesRequest, error) {
	_, vol, _ := trades.GetTradeStatsItems()
	clsTrade, err := NewCloseTrade(id, trades, timeframe, timestamp, requestedPrice, float64(vol))
	if err != nil {
		return nil, fmt.Errorf("NewCloseTradesRequest: failed to create new close trade: %w", err)
	}

	return []*CloseTradeRequestV1{
		{
			Trade:     clsTrade,
			Timeframe: timeframe,
			Volume:    float64(-vol),
			Reason:    reason,
		},
	}, nil
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
func (r *BulkCloseRequest) Execute(price float64, symbol string, timeframe int) ([]*Trade, error) {
	trades := make([]*Trade, 0)
	for _, it := range r.Items {
		if it.ClosePercent < 0 || it.ClosePercent > 1 {
			return nil, InvalidClosePercentErr
		}

		if it.Level.Trades != nil {
			_, vol, _ := it.Level.Trades.GetTradeStatsItems()
			closeVol := float64(vol) * it.ClosePercent * -1
			tr, err := NewOpenTrade(uuid.New(), TradeTypeClose, symbol, timeframe, time.Now(), price, closeVol, 0)
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
	Timeframe int
	Strategy  *Strategy
}

func NewOpenTradeRequest(timeframe int, strategy *Strategy) (*OpenTradeRequest, error) {
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

	if r.Timeframe <= 0 {
		return InvalidTimeframeErr
	}

	return nil
}
