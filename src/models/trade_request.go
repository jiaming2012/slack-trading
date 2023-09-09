package models

import (
	"fmt"
	"github.com/google/uuid"
	"math"
	"time"
)

type CloseTradesRequest []*CloseTradeRequest

type CloseTradeRequest struct {
	Trade  *Trade
	Volume float64
	Reason string
}

func (r *CloseTradeRequest) Validate() error {
	if r.Reason == "" {
		return fmt.Errorf("CloseTradeRequest: reason was not set")
	}

	if math.Abs(r.Volume) == 0 {
		return TradeVolumeIsZeroErr
	}

	if r.Trade == nil {
		return fmt.Errorf("CloseTradeRequest: closing trade not set")
	}

	if math.Abs(r.Trade.ExecutedVolume) == 0 {
		return fmt.Errorf("CloseTradeRequest: closing trade executed volume is zero")
	}

	if math.Abs(r.Volume) > math.Abs(r.Trade.ExecutedVolume) {
		return fmt.Errorf("CloseTradeRequest: volume of close request cannot exceed trade volume")
	}

	return nil
}

func NewCloseTradeRequest(percent float64, reason string, priceLevel *PriceLevel) {

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
func (r *BulkCloseRequest) Execute(price float64, symbol string) ([]*Trade, error) {
	trades := make([]*Trade, 0)
	for _, it := range r.Items {
		if it.ClosePercent < 0 || it.ClosePercent > 1 {
			return nil, InvalidClosePercentErr
		}

		if it.Level.Trades != nil {
			_, vol, _ := it.Level.Trades.Vwap()
			closeVol := float64(vol) * it.ClosePercent * -1
			newTrade, err := NewTradeOpen(uuid.New(), TradeTypeClose, symbol, time.Now(), price, closeVol, 0)
			if err != nil {
				return nil, fmt.Errorf("BulkCloseRequest.Execute: failed to open NewTrade: %w", err)
			}

			newTrade.RequestedVolume = closeVol

			it.Level.Trades.Add(newTrade)
			trades = append(trades, newTrade)
		}
	}

	return trades, nil
}

type OpenTradeRequest struct {
	Symbol   string
	Volume   float64
	Type     TradeType
	Price    float64
	StopLoss float64
	Strategy *Strategy
}

func NewOpenTradeRequest(symbol string, tradeType TradeType, volume float64, price float64, stopLoss float64) (*OpenTradeRequest, error) {
	request := &OpenTradeRequest{
		Symbol:   symbol,
		Volume:   volume,
		Type:     tradeType,
		Price:    price,
		StopLoss: stopLoss,
	}

	if err := request.Validate(); err != nil {
		return nil, fmt.Errorf("NewOpenTradeRequest validation failed: %w", err)
	}

	return request, nil
}

func (r *OpenTradeRequest) Validate() error {
	if r.Symbol == "" {
		return SymbolNotSetErr
	}

	if r.Volume > 0 {
		if r.StopLoss >= r.Price {
			return fmt.Errorf("%w: stopLoss of %v is above current price of %v", InvalidStopLossErr, r.StopLoss, r.Price)
		}
	} else if r.Volume < 0 {
		if r.StopLoss > 0 && r.StopLoss <= r.Price {
			return fmt.Errorf("%w: stopLoss of %v is below current price of %v", InvalidStopLossErr, r.StopLoss, r.Price)
		}
	} else {
		return TradeVolumeIsZeroErr
	}

	if r.Type != TradeTypeBuy && r.Type != TradeTypeSell && r.Type != TradeTypeClose {
		return UnknownTradeTypeErr
	}

	if r.Price <= 0 {
		return InvalidRequestedPriceErr
	}

	if r.StopLoss <= 0 {
		return NonPositiveStopLossErr
	}

	return nil
}
