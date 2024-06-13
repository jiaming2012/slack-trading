package eventmodels

import "strings"

type StockSymbol string

func (s StockSymbol) String() string {
	return strings.ToLower(string(s))
}