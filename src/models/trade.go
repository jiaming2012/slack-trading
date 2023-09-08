package models

import (
	"fmt"
	"github.com/google/uuid"
	"strconv"
	"time"
)

type TradeType int

const (
	TradeTypeBuy TradeType = iota
	TradeTypeSell
	TradeTypeClose
	TradeTypeUnknown
)

type Profit struct {
	Floating float64
	Realized float64
	Volume   Volume
}

type TradeParameters struct {
	PriceLevel *PriceLevel
	MaxLoss    float64
}

type Trade struct {
	ID              uuid.UUID
	Type            TradeType
	Symbol          string
	Timestamp       time.Time
	RequestedVolume float64
	ExecutedVolume  float64
	ExecutedPrice   float64
	RequestedPrice  float64
	StopLoss        float64
	Closes          []*Trade
}

type ClosePercent float64

func (p ClosePercent) Validate() error {
	if p <= 0 || p > 1 {
		return InvalidClosePercentErr
	}

	return nil
}

func (tr *Trade) Side() TradeType {
	if tr.RequestedVolume > 0 {
		return TradeTypeBuy
	}

	if tr.RequestedVolume < 0 {
		return TradeTypeSell
	}

	return TradeTypeUnknown
}

func (tr Trade) String() string {
	volumeStr := strconv.FormatFloat(tr.RequestedVolume, 'f', -1, 64)
	return fmt.Sprintf("%s %s @%.2f", volumeStr, tr.Symbol, tr.ExecutedPrice)
}

func (tr *Trade) Validate() error {
	if tr.ID == uuid.Nil {
		return NoTradeIDErr
	}

	if tr.Symbol == "" {
		return NoSymbolErr
	}

	if tr.Type != TradeTypeBuy && tr.Type != TradeTypeSell && tr.Type != TradeTypeClose {
		return UnknownTradeTypeErr
	}

	if tr.Timestamp.IsZero() {
		return NoTimestampErr
	}

	if tr.RequestedPrice <= 0 {
		return InvalidRequestedPriceErr
	}

	if tr.StopLoss < 0 {
		return NegativeStopLossErr
	}

	if tr.Type != TradeTypeClose && tr.StopLoss == 0 {
		return NoStopLossErr
	}

	if tr.RequestedVolume > 0 {
		if tr.StopLoss >= tr.RequestedPrice {
			return fmt.Errorf("%w: stopLoss of %v is above current price of %v", InvalidStopLossErr, tr.StopLoss, tr.RequestedPrice)
		}
	} else if tr.RequestedVolume < 0 {
		if tr.StopLoss > 0 && tr.StopLoss <= tr.RequestedPrice {
			return fmt.Errorf("%w: stopLoss of %v is below current price of %v", InvalidStopLossErr, tr.StopLoss, tr.RequestedPrice)
		}
	} else {
		return TradeVolumeIsZeroErr
	}

	return nil
}

// Execute sets the actual price that the trade was executed at when sending the trade to the market
func (tr *Trade) Execute(executedPrice float64, executedVolume float64) {
	tr.ExecutedPrice = executedPrice
	tr.ExecutedVolume = executedVolume
}

// AutoExecute sets the executed price to the requested price
func (tr *Trade) AutoExecute() {
	tr.ExecutedPrice = tr.RequestedPrice
	tr.ExecutedVolume = tr.RequestedVolume
}

func NewTrade(id uuid.UUID, tradeType TradeType, symbol string, timestamp time.Time, requestedPrice float64, requestedVolume float64, stopLoss float64) (*Trade, error) {
	trade := &Trade{
		ID:              id,
		Symbol:          symbol,
		Type:            tradeType,
		Timestamp:       timestamp,
		RequestedPrice:  requestedPrice,
		RequestedVolume: requestedVolume,
		StopLoss:        stopLoss,
	}

	if err := trade.Validate(); err != nil {
		return nil, fmt.Errorf("NewTrade: failed to open new trade: %w", err)
	}

	return trade, nil
}
