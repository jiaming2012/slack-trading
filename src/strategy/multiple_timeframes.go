package strategy

import (
	"context"
	"fmt"
	"github.com/kataras/go-events"
	log "github.com/sirupsen/logrus"
	"slack-trading/src/indicators"
	"slack-trading/src/models"
	"slack-trading/src/sheets"
)

var rsiM5 *indicators.Rsi

var rsiM15 *indicators.Rsi

var candles models.Candles

func newCandleHandler(payload ...interface{}) {
	candle := payload[0].(*models.Candle)
	candles.Add(candle)

	events.Emit(models.M5CandleUpdate)
}

func m5RsiHandler(payload ...interface{}) {
	rsi := indicators.NewRsi(14)
	var val float64

	for _, c := range candles.Data {
		val = rsi.Update(c)
	}

	fmt.Println("M5 RSI: ", val)
}

func m15RsiHandler(payload ...interface{}) {
	m15Candles, err := candles.ConvertTo(15)
	if err != nil {
		log.Errorf("m15RsiHandler: %v", err)
		return
	}

	rsi := indicators.NewRsi(14)
	var val float64

	for _, c := range m15Candles.Data {
		val = rsi.Update(c)
	}

	fmt.Println("M15 RSI: ", val)
}

func Worker() {
	ctx := context.Background()
	fetched, err := sheets.FetchCandles(ctx)
	if err != nil {
		log.Fatalf("failed to initiate worker: %v", err)
	}

	candles = *fetched
	rsiM5 = indicators.NewRsi(14)
	events.On(models.NewM5Candle, newCandleHandler)
	events.On(models.M5CandleUpdate, m5RsiHandler)
	events.On(models.M5CandleUpdate, m15RsiHandler)
}
