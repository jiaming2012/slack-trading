package models

type PlaceEquityTradeRequest struct {
	Symbol    string
	Quantity  int
	Side      TradierOrderSide
	OrderType TradierOrderType
	Tag       string
	DryRun    bool
}

func NewPlaceEquityOrderRequest(symbol string, quantity int, side TradierOrderSide, orderType OrderRecordType, tag string, dryRun bool) *PlaceEquityTradeRequest {
	return &PlaceEquityTradeRequest{
		Symbol:    symbol,
		Quantity:  quantity,
		Side:      side,
		OrderType: TradierOrderTypeMarket,
		Tag:       tag,
		DryRun:    dryRun,
	}
}
