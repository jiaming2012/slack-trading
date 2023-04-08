package strategy

import (
	"fmt"
	"github.com/kataras/go-events"
	"slack-trading/src/indicators"
	"slack-trading/src/models"
)

var rsiM5 *indicators.Rsi

func newM5CandleHandler(payload ...interface{}) {
	c := payload[0].(*models.Candle)

	rsi := rsiM5.Update(*c)

	fmt.Println("M5 RSI: ", rsi)
}

func Worker() {
	rsiM5 = indicators.NewRsi(14)
	events.On(models.NewM5Candle, newM5CandleHandler)
}
