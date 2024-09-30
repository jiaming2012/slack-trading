package models

import "fmt"

type BacktesterOrderClass string

const (
	Equity BacktesterOrderClass = "equity"
)

func (c BacktesterOrderClass) Validate() error {
	switch c {
	case Equity:
		return nil
	default:
		return fmt.Errorf("invalid order class: %s", c)
	}
}
