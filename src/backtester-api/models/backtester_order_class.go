package models

import "fmt"

type BacktesterOrderClass string

const (
	BacktesterOrderClassEquity BacktesterOrderClass = "equity"
)

func (c BacktesterOrderClass) Validate() error {
	switch c {
	case BacktesterOrderClassEquity:
		return nil
	default:
		return fmt.Errorf("invalid order class: %s", c)
	}
}
