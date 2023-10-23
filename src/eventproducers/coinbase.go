package eventproducers

import (
	"context"
	"fmt"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"slack-trading/src/worker"
	"sync"
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
	go worker.Run(workerContext, ch)

	go func() {
		defer r.wg.Done()
		for {
			select {
			case <-workerContext.Done():
				// todo: reduce log level
				log.Errorf("Coinbase worker stopped. Resetting worker context ...")
				workerContext = context.Background()
				go worker.Run(workerContext, ch)
			case <-ctx.Done():
				fmt.Printf("\nstopping Coinbase producer\n")
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
