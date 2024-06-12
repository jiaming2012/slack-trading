package eventproducers

import (
	"context"
	"sync"
	"time"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"

	"slack-trading/src/worker"
)

type coinbaseClient struct {
	wg     *sync.WaitGroup
	router *mux.Router
}

func (r *coinbaseClient) Start(ctx context.Context) {
	r.wg.Add(1)

	// setup coinbase worker
	ch := make(chan worker.CoinbaseDTO)
	workerContext := context.Background()

	// connect to coinbase
	c, ConnErr := worker.Connect()
	if ConnErr != nil {
		log.Fatal("coinbase: initial connect failed:", ConnErr)
	}

	defer c.Close()

	go worker.Run(workerContext, ch, c)

	go func() {
		defer r.wg.Done()
		for {
			select {
			case <-workerContext.Done():
				// todo: reduce log level
				log.Errorf("Coinbase worker stopped. Resetting worker context ...")
				workerContext = context.Background()

				// reconnect to coinbase
				c, ConnErr = worker.Connect()
				if ConnErr != nil {
					log.Error("coinbase: initial connect failed:", ConnErr)
					log.Info("retrying in 5 seconds ...")
					time.Sleep(5 * time.Second)
					continue
				}

				go worker.Run(workerContext, ch, c)
			case <-ctx.Done():
				log.Infof("stopping Coinbase producer")
				return
			}
		}
	}()
}

func NewCoinbaseClient(wg *sync.WaitGroup, router *mux.Router) *coinbaseClient {
	return &coinbaseClient{
		wg:     wg,
		router: router,
	}
}
