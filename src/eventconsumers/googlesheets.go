package eventconsumers

import (
	"context"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	models "slack-trading/src/eventmodels"
	pubsub "slack-trading/src/eventpubsub"
	models2 "slack-trading/src/models"
	"slack-trading/src/sheets"
	"sync"
)

type GoogleSheetsClient struct {
	ctx context.Context
	wg  *sync.WaitGroup
}

func (c *GoogleSheetsClient) writeToCSV(tradeFulfilledEvent models.TradeFulfilledEvent) {
	log.Debugf("GoogleSheetsClient.writeToCSV <- %v", tradeFulfilledEvent)

	err := sheets.AppendTrade(c.ctx, &models2.Trade{
		ID:             uuid.New(),
		Symbol:         tradeFulfilledEvent.Symbol,
		Time:           tradeFulfilledEvent.Timestamp,
		Volume:         tradeFulfilledEvent.Volume,
		ExecutedPrice:  tradeFulfilledEvent.ExecutedPrice,
		RequestedPrice: tradeFulfilledEvent.RequestedPrice,
		StopLoss:       0,
	})

	if err != nil {
		panic(err)
	}
}

func (c *GoogleSheetsClient) Start() {
	c.wg.Add(1)

	pubsub.Subscribe("GoogleSheetsClient", pubsub.TradeFulfilledEvent, c.writeToCSV)

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
