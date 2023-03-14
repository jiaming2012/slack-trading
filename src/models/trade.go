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
}

type Trade struct {
	Symbol         string
	Time           time.Time
	Volume         float64
	RequestedPrice float64
	ExecutedPrice  float64
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
