package notifier

import "slack-trading/src/eventconsumers/unpure/notifier/models"

type Notifier interface {
	CompileRealizedProfitLossEvent(models.RealizedProfitLossEvent) string
	CompileTradeFulfilledEvent(models.TradeFulfilledEvent) string
}

// stateless ???
type notifier struct{}

// Compile is testable
func (n notifier) Compile(ev models.RealizedProfitLossEvent) string {
	// so slack stuff
	return ""
}

// Do is an actual mutation
func (n notifier) Do() {

}

func NewNotifier() *notifier {
	return &notifier{}
}
