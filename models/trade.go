package models

import (
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
	Symbol string
	Time   time.Time
	Volume float64
	Price  float64
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

//func (tr *Trade) GetProfit(currentPrice float64) float64 {
//	switch tr.Side() {
//	case TradeTypeBuy:
//		return tr.Volume * (currentPrice - tr.Price)
//	case TradeTypeSell:
//		return math.Abs(tr.Volume) * (tr.Price - currentPrice)
//	default:
//		panic(fmt.Errorf("GetProfit not implemented for %v", tr.Side()))
//	}
//}
