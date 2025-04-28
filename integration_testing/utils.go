package integrationtesting

import (
	"context"
	"fmt"
	"time"

	"github.com/jiaming2012/slack-trading/src/playground"
)

func waitUntilOrderStatus(p playground.PlaygroundService, orderId uint64, expectedStatus string) error {
	ctx := context.Background()
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timed out waiting for order %d to reach status %s", orderId, expectedStatus)
		case <-ticker.C:
			orderResp, err := p.GetOrder(ctx, &playground.GetOrderRequest{
				OrderId: orderId,
			})

			if err != nil {
				return fmt.Errorf("error fetching order %d: %v", orderId, err)
			}

			if orderResp.Status == expectedStatus {
				return nil
			}
		}
	}
}
