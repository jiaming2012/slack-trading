package eventconsumers

import (
	"context"
	"sync"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"slack-trading/src/eventmodels"
	pubsub "slack-trading/src/eventpubsub"
	modelsV1 "slack-trading/src/models"
	"slack-trading/src/sheets"
)

type GoogleSheetsClient struct {
	ctx context.Context
	wg  *sync.WaitGroup
}

func (c *GoogleSheetsClient) writeTradeToCSV(tradeFulfilledEvent eventmodels.TradeFulfilledEvent) {
	log.Debugf("GoogleSheetsClient.writeToCSV <- %v", tradeFulfilledEvent)

	err := sheets.AppendTrade(c.ctx, &modelsV1.Trade{
		ID:              uuid.New(),
		Symbol:          tradeFulfilledEvent.Symbol,
		Timestamp:       tradeFulfilledEvent.Timestamp,
		RequestedVolume: tradeFulfilledEvent.Volume,
		ExecutedPrice:   tradeFulfilledEvent.ExecutedPrice,
		RequestedPrice:  tradeFulfilledEvent.RequestedPrice,
		StopLoss:        0,
	})

	if err != nil {
		pubsub.PublishError("GoogleSheetsClient.writeTradeToCSV", err)
	}
}

func (c *GoogleSheetsClient) writeCandleToCSV(candle eventmodels.Candle) {
	log.Debugf("GoogleSheetsClient.writeCandleToCSV <- %v", candle)

	// todo: no need to go from Candle -> eventmodels.Candle -> Candle
	err := sheets.AppendCandle(c.ctx, &modelsV1.Candle{
		Timestamp:   candle.Timestamp,
		LastUpdated: candle.LastUpdated,
		Open:        candle.Open,
		High:        candle.High,
		Low:         candle.Low,
		Close:       candle.Close,
	})

	if err != nil {
		pubsub.PublishError("GoogleSheetsClient.writeCandleToCSV", err)
	}
}

func (c *GoogleSheetsClient) Start() {
	c.wg.Add(1)

	pubsub.Subscribe("GoogleSheetsClient", eventmodels.TradeFulfilledEventName, c.writeTradeToCSV)
	pubsub.Subscribe("GoogleSheetsClient", eventmodels.NewCandleEventName, c.writeCandleToCSV)

	go func() {
		defer c.wg.Done()
		for {
			select {
			case <-c.ctx.Done():
				log.Info("stopping GoogleSheetsClient consumer")
				return
			}
		}
	}()
}

func NewGoogleSheetsClient(ctx context.Context, wg *sync.WaitGroup) *GoogleSheetsClient {
	return &GoogleSheetsClient{
		ctx: ctx,
		wg:  wg,
	}
}
