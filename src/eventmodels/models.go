package eventmodels

import "fmt"

type ReportEvent struct {
	Data string
}

type NewTradeRequestEvent struct {
	Symbol string
}

func (ev NewTradeRequestEvent) String() string {
	return fmt.Sprintf("NewTradeRequestEvent: %v", ev.Symbol)
}
