package eventmodels

type PlaceTradeSpreadRequest struct {
	Underlying       StockSymbol
	Spread           *OptionSpreadContractDTO
	Quantity         int
	TradeType        TradierTradeType
	Price            float64
	TradeDuration    TradeDuration
	Tag              string
	MaxNoOfPositions int
}
