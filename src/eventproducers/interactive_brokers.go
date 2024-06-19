package eventproducers

import (
	"context"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/worker"
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

	conn, ConnErr := worker.IBConnect(info.ServerURL, info.ConnID)
	if ConnErr != nil {
		log.Fatalf("IBTickListener: initial connect failed: %v", ConnErr)
	}

	defer conn.Close()

	go worker.IBTickListener(workerContext, info, ch, conn)

	go func() {
		defer c.wg.Done()
		for {
			select {
			// todo: add logic to reset if heartbeat is not received
			case <-workerContext.Done():
				log.Errorf("IB worker stopped. Resetting worker context ...")
				workerContext = context.Background()

				conn, ConnErr := worker.IBConnect(info.ServerURL, info.ConnID)
				if ConnErr != nil {
					log.Fatalf("IBTickListener: initial connect failed: %v", ConnErr)
				}

				go worker.IBTickListener(workerContext, info, ch, conn)
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
