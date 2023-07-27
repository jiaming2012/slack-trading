package eventconsumers

import (
	"context"
	"fmt"
	log "github.com/sirupsen/logrus"
	"slack-trading/src/eventmodels"
	"slack-trading/src/eventpubsub"
	"sync"
)

type TradeExecutor struct {
	wg *sync.WaitGroup
}

func (r *TradeExecutor) executeTrade(e eventmodels.NewTradeRequestEvent) {
	log.Debugf("TradeExecutor.executeTrade -> %v", e)
}

func (r *TradeExecutor) Start(ctx context.Context) {
	r.wg.Add(1)

	eventpubsub.Subscribe(eventpubsub.NewTradeRequestEvent, r.executeTrade)

	go func() {
		defer r.wg.Done()
		for {
			select {
			case <-ctx.Done():
				fmt.Printf("\nstopping TradeExecutor consumer\n")
				return
			}
		}
	}()
}

func NewTradeExecutorClient(wg *sync.WaitGroup) *TradeExecutor {
	return &TradeExecutor{
		wg: wg,
	}
}