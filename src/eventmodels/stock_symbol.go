package eventmodels

import (
	"encoding/json"
	"strings"
)

type StockSymbol string

func (s StockSymbol) GetTicker() string {
	return s.String()
}

func (s StockSymbol) String() string {
	return strings.ToUpper(string(s))
}

func (s StockSymbol) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

func NewStockSymbol(s string) StockSymbol {
	return StockSymbol(strings.ToUpper(s))
}