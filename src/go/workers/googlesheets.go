package workers

import (
	"context"
	"sync"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/go/models"
	pubsub "github.com/jiaming2012/slack-trading/src/go/pubsub"
	"github.com/jiaming2012/slack-trading/src/go/sheets"
)

type GoogleSheetsClient struct {
	ctx context.Context
	wg  *sync.WaitGroup
}

func (c *GoogleSheetsClient) writeTradeToCSV(tradeFulfilledEvent models.TradeFulfilledEvent) {
	log.Debugf("GoogleSheetsClient.writeToCSV <- %v", tradeFulfilledEvent)

	err := sheets.AppendTrade(c.ctx, &models.Trade{
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

func (c *GoogleSheetsClient) writeCandleToCSV(candle models.Candle) {
	log.Debugf("GoogleSheetsClient.writeCandleToCSV <- %v", candle)

	// todo: no need to go from Candle -> models.Candle -> Candle
	err := sheets.AppendCandle(c.ctx, &models.Candle{
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

	pubsub.Subscribe("GoogleSheetsClient", models.TradeFulfilledEventName, c.writeTradeToCSV)
	pubsub.Subscribe("GoogleSheetsClient", models.NewCandleEventName, c.writeCandleToCSV)

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
