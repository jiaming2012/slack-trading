package eventproducers

import (
	"context"
	"sync"

	log "github.com/sirupsen/logrus"

	"slack-trading/src/worker"
)

type IBClient struct {
	wg        *sync.WaitGroup
	serverURL string
}

func (c *IBClient) Start(ctx context.Context, symbol string) {
	c.wg.Add(1)

	ch := make(chan worker.IBTickDTO)
	workerContext := context.Background()

	// todo: set in config
	var conId string
	switch symbol {
	case "CL":
		conId = "212921504"
	default:
		panic("invalid symbol")
	}

	info := worker.IBTickInfo{
		ConnID:    conId,
		ServerURL: c.serverURL,
	}

	go worker.IBTickListener(workerContext, info, ch)

	go func() {
		defer c.wg.Done()
		for {
			select {
			// todo: add logic to reset if heartbeat is not received
			case <-workerContext.Done():
				log.Errorf("IB worker stopped. Resetting worker context ...")
				workerContext = context.Background()
				go worker.IBTickListener(workerContext, info, ch)
			case <-ctx.Done():
				log.Infof("stopping IB producer")
				return
			}
		}
	}()

}

func NewIBClient(wg *sync.WaitGroup, serverURL string) *IBClient {
	return &IBClient{
		wg:        wg,
		serverURL: serverURL,
	}
}
