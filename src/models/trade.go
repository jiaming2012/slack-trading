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
	TradeTypeUnknown
)

type Profit struct {
	Floating float64
	Realized float64
	Volume   Volume
}

type TradeRequest struct {
	Symbol   string
	Type     TradeType
	Price    float64
	StopLoss float64
}

type TradeParameters struct {
	PriceLevel *PriceLevel
	MaxLoss    float64
}

type Trade struct {
	ID             uuid.UUID
	Symbol         string
	Time           time.Time
	Volume         float64
	ExecutedPrice  float64
	RequestedPrice float64
	StopLoss       float64
}

type ClosePercent float64

func (p ClosePercent) Validate() error {
	if p <= 0 || p > 1 {
		return InvalidClosePercentErr
	}

	return nil
}

func (tr *Trade) Side() TradeType {
	if tr.Volume > 0 {
		return TradeTypeBuy
	}

	if tr.Volume < 0 {
		return TradeTypeSell
	}

	return TradeTypeUnknown
}

func (tr Trade) String() string {
	volumeStr := strconv.FormatFloat(tr.Volume, 'f', -1, 64)
	return fmt.Sprintf("%s %s @%.2f", volumeStr, tr.Symbol, tr.ExecutedPrice)
}

func (tr *Trade) Validate(stopLoss bool) error {
	if stopLoss {
		if tr.StopLoss <= 0 {
			return NoStopLossErr
		}

		if tr.Volume > 0 {
			if tr.StopLoss >= tr.RequestedPrice {
				return fmt.Errorf("%w: stopLoss of %v is above current price of %v", InvalidStopLossErr, tr.StopLoss, tr.RequestedPrice)
			}
		} else if tr.Volume < 0 {
			if tr.StopLoss <= tr.RequestedPrice {
				return fmt.Errorf("%w: stopLoss of %v is below current price of %v", InvalidStopLossErr, tr.StopLoss, tr.RequestedPrice)
			}
		} else {
			return TradeVolumeIsZeroErr
		}
	}

	return nil
}

// Execute sets the actual price that the trade was executed at when sending the trade to the market
func (tr *Trade) Execute(executedPrice float64) {
	tr.ExecutedPrice = executedPrice
}

// AutoExecute sets the executed price to the requested price
func (tr *Trade) AutoExecute() {
	tr.ExecutedPrice = tr.RequestedPrice
}

func NewTrade(requestedPrice float64) *Trade {
	return &Trade{
		ID:             uuid.New(),
		Symbol:         "BTCUSD",
		Time:           time.Now(),
		RequestedPrice: requestedPrice,
	}
}
