package notifier

import (
	"fmt"
	"slack-trading/src/eventconsumers/unpure/notifier/models"
)

type mockNotifier struct{}

func (m mockNotifier) CompileRealizedProfitLossEvent(ev models.RealizedProfitLossEvent) string {
	return fmt.Sprintf("Realized PL: $%.2f", ev.Profit)
}

func (m mockNotifier) CompileTradeFulfilledEvent(ev models.TradeFulfilledEvent) string {
	var direction string
	if ev.Volume > 0 {
		direction = "+"
	} else if ev.Volume < 0 {
		direction = "-"
	} else {
		direction = ""
	}

	timestamp := ev.Timestamp.String()

	return fmt.Sprintf("%s: %s%.8f %s @ $%.2f", timestamp, direction, ev.Volume, ev.Symbol, ev.Price)
}

func NewMockNotifier() *mockNotifier {
	return &mockNotifier{}
}
