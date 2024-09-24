package models

type BacktesterOrderSide string

const (
	BacktesterOrderSideBuy        BacktesterOrderSide = "buy"
	BacktesterOrderSideSell       BacktesterOrderSide = "sell"
	BacktesterOrderSideBuyToCover BacktesterOrderSide = "buy_to_cover"
	BacktesterOrderSideSellShort  BacktesterOrderSide = "sell_short"
)
