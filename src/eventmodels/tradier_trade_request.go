package eventmodels

type TradierTradeRequest struct {
	Underlying StockSymbol
	BuyToOpen  OptionSymbol
	SellToOpen OptionSymbol
	Quantity   int
	Tag        string
}
