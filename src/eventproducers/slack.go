package eventproducers

import (
	"context"
	"fmt"
	"github.com/gorilla/mux"
	"slack-trading/src/eventproducers/slack"
	"sync"
)

type client struct {
	wg     *sync.WaitGroup
	router *mux.Router
}

func (r *client) Start(ctx context.Context) {
	r.wg.Add(1)

	r.router.HandleFunc("/", slack.Handler)

	go func() {
		defer r.wg.Done()
		for {
			select {
			case <-ctx.Done():
				fmt.Printf("\nstopping Slack producer\n")
				return
			}
		}
	}()
}

func NewSlackClient(wg *sync.WaitGroup, router *mux.Router) *client {
	return &client{
		wg:     wg,
		router: router,
	}
}
