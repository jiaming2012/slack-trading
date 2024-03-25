package eventmodels

import "time"

type OptionQuote struct {
	Symbol         string
	LastPrice      float64
	Underlying     string
	Delta          float64
	ExpirationDate time.Time
}
