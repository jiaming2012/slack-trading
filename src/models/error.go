package models

import "fmt"

var BalanceOutOfRangeErr = fmt.Errorf("balance is out of range")
var LevelsNotSetErr = fmt.Errorf("at least two price levels must be set")
var MaxLossPercentErr = fmt.Errorf("maxLossPercentage must be a value between 0 and 1")
var PriceLevelsNotSortedErr = fmt.Errorf("price levels are not sorted")
var PriceOutsideLimitsErr = fmt.Errorf("price is outside price limits")
var MaxTradesPerPriceLevelErr = fmt.Errorf("too many trades placed in current price level")
var PriceLevelsAllocationErr = fmt.Errorf("invalid price levels allocation")
var InvalidStopLossErr = fmt.Errorf("invalid stop loss")
var NoStopLossErr = fmt.Errorf("stop loss not set")
var TradeVolumeIsZeroErr = fmt.Errorf("trade volume must be non zero")

type ErrorDTO struct {
	Msg string `json:"msg"`
}
