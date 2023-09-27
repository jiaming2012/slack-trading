package models

import "fmt"

var NoRequestParams = fmt.Errorf("no request params found")
var BalanceOutOfRangeErr = fmt.Errorf("balance is out of range")
var MaxLossPercentErr = fmt.Errorf("maxLossPercentage must be a value between 0 and 1")
var PriceLevelsNotSortedErr = fmt.Errorf("price levels are not sorted")
var PriceOutsideLimitsErr = fmt.Errorf("price is outside price limits")
var InvalidMaxTradesErr = fmt.Errorf("max trades must be greater or equal to zero")
var MaxTradesPerPriceLevelErr = fmt.Errorf("too many trades placed in current price level")
var PriceLevelsAllocationErr = fmt.Errorf("invalid price levels allocation")
var InvalidAllocationPercentErr = fmt.Errorf("allocation percent must be >= 0 and <= 1")
var PriceLevelsLastAllocationErr = fmt.Errorf("the last price level must have an allocation of zero")
var MinimumNumberOfPriceLevelsNotMetErr = fmt.Errorf("price levels must have at least two levels")
var MaxLossPriceBandErr = fmt.Errorf("the max loss within this price band has already been achieved")
var InvalidStopLossErr = fmt.Errorf("invalid stop loss")
var NoStopLossErr = fmt.Errorf("stop loss not set for all non closing trades")
var NegativeStopLossErr = fmt.Errorf("stop loss must be greater than or equal to zero")
var NegativePriceErr = fmt.Errorf("price must be greater than or equal to zero")
var NonPositiveStopLossErr = fmt.Errorf("stop loss must be greater than zero")
var SymbolNotSetErr = fmt.Errorf("symbol is not set")
var UnknownTradeTypeErr = fmt.Errorf("trade type is not set")
var NoTradeIDErr = fmt.Errorf("trade ID is not set")
var NoTimestampErr = fmt.Errorf("timestamp is not set")
var InvalidRequestedPriceErr = fmt.Errorf("requested price must be a positive number")
var NonPositiveStopLoss = fmt.Errorf("stop loss is less than or equal to zero")
var TradeVolumeIsZeroErr = fmt.Errorf("trade volume must be non zero")
var NoOfTradeMustBeNonzeroErr = fmt.Errorf("number of trades for level with allocation must be greater than zero")
var NoOfTradesMustBeZeroErr = fmt.Errorf("number of trades for a level with allocation of zero must also be zero")
var NoClosePercentSetErr = fmt.Errorf("closing trades must have a closePercent set")
var InvalidClosePercentErr = fmt.Errorf("close percent value must be be > 0 and <= 1")

//var DuplicateCloseTradeErr = fmt.Errorf("volume of closing trade must be less than or equal to the sum of offset trade's volume")
var BalanceGreaterThanZeroErr = fmt.Errorf("balance must be greater than zero")
var OffsetTradesVolumeExceedsClosingTradeVolumeErr = fmt.Errorf("the sum of N-1 offsetting trades volume cannot be greater or equal to the closing trades volume")
var NoOffsettingTradeErr = fmt.Errorf("closing trades must have at least one offsetting trade")
var InvalidTimeframeErr = fmt.Errorf("timeframe must be greater than zero")
var NoRemainingRiskAvailableErr = fmt.Errorf("cannot open trade because no risk is available")
var PriceLevelMinimumDistanceNotSatisfiedError = fmt.Errorf("price level minimum distance condition not met")
var PriceLevelStopLossMustBeOutsideLowerAndUpperRangeErr = fmt.Errorf("sl of price level must be less than the lower level and greater than the upper level")
var InvalidPriceLevelIndexErr = fmt.Errorf("price level index must be greater than or equal to zero")
var PartialCloseItemNotSetErr = fmt.Errorf("partial close item was not set on offsetting trade. This is most likely an internal error")
var DuplicateCloseTradeErr = fmt.Errorf("trade already closed")

type ErrorDTO struct {
	Msg string `json:"msg"`
}
