package models

import (
	"testing"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/stretchr/testify/require"
)

func Test_TradierOrderDataStore(t *testing.T) {
	t.Run("add an order", func(t *testing.T) {
		// arrange
		orders := NewTradierOrderDataStore()
		order := &eventmodels.TradierOrder{
			ID: 1,
		}

		// act
		orders.Add(order)

		// assert
		require.Equal(t, 1, len(orders))
		require.Equal(t, order, orders[order.ID])
	})

	t.Run("delete an order", func(t *testing.T) {
		// arrange
		orders := NewTradierOrderDataStore()
		order := &eventmodels.TradierOrder{
			ID: 1,
		}
		orders.Add(order)

		// act
		orders.Delete(order.ID)

		// assert
		require.Equal(t, 0, len(orders))
	})

	t.Run("update an order", func(t *testing.T) {
		// arrange
		orders := NewTradierOrderDataStore()
		order := &eventmodels.TradierOrder{
			ID:     1,
			Status: "open",
		}
		orders.Add(order)

		// act
		update := &eventmodels.TradierOrder{
			ID:     1,
			Status: "filled",
		}

		updates := orders.Update(update)

		// assert
		require.Equal(t, 1, len(updates))
		require.Equal(t, "status", updates[0].Field)
		require.Equal(t, "open", updates[0].Old)
		require.Equal(t, "filled", updates[0].New)
		require.Equal(t, "filled", orders[order.ID].Status)
	})

	t.Run("update an order that does not exist", func(t *testing.T) {
		// arrange
		orders := NewTradierOrderDataStore()

		// act
		update := &eventmodels.TradierOrder{
			ID:     1,
			Status: "filled",
		}

		updates := orders.Update(update)

		// assert
		require.Equal(t, 0, len(updates))
	})

	t.Run("fail to update an order with mismatch ID", func(t *testing.T) {
		// arrange
		orders := NewTradierOrderDataStore()
		order := &eventmodels.TradierOrder{
			ID:     1,
			Status: "open",
		}
		orders.Add(order)

		// act
		update := &eventmodels.TradierOrder{
			ID:     2,
			Status: "filled",
		}

		updates := orders.Update(update)

		// assert
		require.Equal(t, 0, len(updates))
		require.Equal(t, len(orders), 1)
		require.Equal(t, "open", orders[order.ID].Status)
	})
}
