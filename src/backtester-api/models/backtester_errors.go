package models

import "fmt"

var (
	ErrInsufficientFreeMargin = fmt.Errorf("insufficient free margin")
	ErrNoPriceAvailable       = fmt.Errorf("no price available")
)
