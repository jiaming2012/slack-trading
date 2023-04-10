package models

import (
	"fmt"
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

type Trade struct {
	Symbol         string
	Time           time.Time
	Volume         float64
	RequestedPrice float64
	ExecutedPrice  float64
	StopLoss       float64
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

func (tr *Trade) Validate() error {
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

	return nil
}

// Executer set the actual price fulfilled when sending the trade to the market
func (tr *Trade) Execute(executedPrice float64) {
	tr.ExecutedPrice = executedPrice
}
