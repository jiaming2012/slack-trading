package models

import "fmt"

var (
	ErrDbOrderIsNotOpenOrPending = fmt.Errorf("order record is not open or pending")
	ErrTradingNotAllowed         = fmt.Errorf("trading is not allowed: order is not open or partially filled")
	ErrCurrentPriceNotSet        = fmt.Errorf("current price is not set")
	ErrOrderAlreadyFilled        = fmt.Errorf("order is already filled")
)
