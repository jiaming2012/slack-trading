package eventconsumers

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func Test_TradierApiWorker_CheckForDelete(t *testing.T) {
	wg := sync.WaitGroup{}

	t.Run("check for delete returns empty list", func(t *testing.T) {
		// arrange
		worker := NewTradierApiWorker(&wg, "", "", nil, nil, "", nil, nil)
		order := &eventmodels.TradierOrder{
			ID: 1,
		}
		worker.orders.Add(order)

		// act
		fetchedOrders := []*eventmodels.TradierOrderDTO{
			{
				ID: 1,
			},
		}
		deletedOrders := worker.checkForDelete(fetchedOrders)

		// assert
		require.Empty(t, deletedOrders)
	})

	t.Run("check for delete returns list of order IDs", func(t *testing.T) {
		// arrange
		worker := NewTradierApiWorker(&wg, "", "", nil, nil, "", nil, nil)
		order := &eventmodels.TradierOrder{
			ID: 1,
		}
		worker.orders.Add(order)

		// act
		fetchedOrders := []*eventmodels.TradierOrderDTO{}
		deletedOrders := worker.checkForDelete(fetchedOrders)

		// assert
		require.Equal(t, 1, len(deletedOrders))
		require.Equal(t, uint64(1), deletedOrders[0])
	})
}

func Test_TradierOrdersMonitoringWorker_CheckForCreateOrUpdate(t *testing.T) {
	wg := sync.WaitGroup{}

	t.Run("check for create order", func(t *testing.T) {
		// arrange
		worker := NewTradierApiWorker(&wg, "", "", nil, nil, "", nil, nil)
		orders := []*eventmodels.TradierOrderDTO{
			{
				ID:              3,
				CreateDate:      "2021-01-01T00:00:00Z",
				TransactionDate: "2021-01-01T00:00:00Z",
			},
		}

		// act
		newOrderEvents, updateOrderEvents := worker.checkForCreateOrUpdate(orders)

		// assert
		require.Equal(t, 1, len(newOrderEvents))
		require.Equal(t, uint64(3), newOrderEvents[0].Order.ID)
		require.Equal(t, 0, len(updateOrderEvents))
	})

	t.Run("check for update order", func(t *testing.T) {
		// arrange
		worker := NewTradierApiWorker(&wg, "", "", nil, nil, "", nil, nil)
		orders1 := []*eventmodels.TradierOrderDTO{
			{
				ID:              3,
				CreateDate:      "2021-01-01T00:00:00Z",
				TransactionDate: "2021-01-01T00:00:00Z",
				Status:          "open",
			},
		}
		worker.checkForCreateOrUpdate(orders1)

		orders2 := []*eventmodels.TradierOrderDTO{
			{
				ID:              3,
				CreateDate:      "2021-01-01T00:00:00Z",
				TransactionDate: "2021-01-01T00:00:00Z",
				Status:          "filled",
			},
		}

		// act
		newOrderEvents, updateOrderEvents := worker.checkForCreateOrUpdate(orders2)

		// assert
		require.Equal(t, 0, len(newOrderEvents))
		require.Equal(t, 1, len(updateOrderEvents))
		require.Equal(t, uint64(3), updateOrderEvents[0].OrderID)
		require.Equal(t, "status", updateOrderEvents[0].Field)
		require.Equal(t, "open", updateOrderEvents[0].Old)
		require.Equal(t, "filled", updateOrderEvents[0].New)
	})
}
