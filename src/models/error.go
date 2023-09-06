package models

import "fmt"

var NoRequestParams = fmt.Errorf("no request params found")
var BalanceOutOfRangeErr = fmt.Errorf("balance is out of range")
var LevelsNotSetErr = fmt.Errorf("at least two price levels must be set")
var MaxLossPercentErr = fmt.Errorf("maxLossPercentage must be a value between 0 and 1")
var PriceLevelsNotSortedErr = fmt.Errorf("price levels are not sorted")
var PriceOutsideLimitsErr = fmt.Errorf("price is outside price limits")
var MaxTradesPerPriceLevelErr = fmt.Errorf("too many trades placed in current price level")
var PriceLevelsAllocationErr = fmt.Errorf("invalid price levels allocation")
var PriceLevelsLastAllocationErr = fmt.Errorf("the last price level must have an allocation of zero")
var MaxLossPriceBandErr = fmt.Errorf("the max loss within this price band has already been achieved")
var InvalidStopLossErr = fmt.Errorf("invalid stop loss")
var NoStopLossErr = fmt.Errorf("stop loss not set")
var TradeVolumeIsZeroErr = fmt.Errorf("trade volume must be non zero")
var NoOfTradeMustBeNonzeroErr = fmt.Errorf("number of trades for level with allocation must be greater than zero")
var NoOfTradesMustBeZeroErr = fmt.Errorf("number of trades for a level with allocation of zero must also be zero")
var NoClosePercentSetErr = fmt.Errorf("closing trades must have a closePercent set")
var InvalidClosePercentErr = fmt.Errorf("close percent value must be be > 0 and <= 1")

type ErrorDTO struct {
	Msg string `json:"msg"`
}
