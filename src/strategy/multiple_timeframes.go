package strategy

import (
	"context"
	"fmt"

	"github.com/jiaming2012/slack-trading/src/indicators"
	"github.com/jiaming2012/slack-trading/src/models"
	"github.com/jiaming2012/slack-trading/src/sheets"
	"github.com/kataras/go-events"
	log "github.com/sirupsen/logrus"
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
	bollinger := indicators.NewBollingerBands(20, 2.0)
	var val float64
	var lastPrice float64
	var bollingerStat indicators.BollingerBandsStats

	for _, c := range candles.Data {
		val = rsi.Update(c)
		_, _stats, err := bollinger.Update(c)
		if err != nil {
			log.Errorf("bollinger: %v", err)
		}
		bollingerStat = _stats
		lastPrice = c.Close
	}

	fmt.Println("M5 Last ExecutedPrice: ", lastPrice)
	fmt.Println("M5 RSI: ", val)
	fmt.Println("M5 Bollinger: ", bollingerStat.Lower, " - ", bollingerStat.Upper)
}

func m15RsiHandler(payload ...interface{}) {
	m15Candles, err := candles.ConvertTo(15)
	if err != nil {
		log.Errorf("m15RsiHandler: %v", err)
		return
	}

	rsi := indicators.NewRsi(14)
	bollinger := indicators.NewBollingerBands(20, 2.0)
	var val float64
	var lastPrice float64
	var bollingerStat indicators.BollingerBandsStats

	for _, c := range m15Candles.Data {
		val = rsi.Update(c)
		_, _stats, err := bollinger.Update(c)
		if err != nil {
			log.Errorf("bollinger: %v", err)
		}
		bollingerStat = _stats
		lastPrice = c.Close
	}

	fmt.Println("M15 Last ExecutedPrice: ", lastPrice)
	fmt.Println("M15 RSI: ", val)
	fmt.Println("M15 Bollinger: ", bollingerStat.Lower, " - ", bollingerStat.Upper)
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
