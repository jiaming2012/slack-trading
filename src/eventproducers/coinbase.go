package eventproducers

import (
	"context"
	"fmt"
	"github.com/gorilla/mux"
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
	go worker.Run(ctx, ch)

	go func() {
		defer r.wg.Done()
		for {
			select {
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
