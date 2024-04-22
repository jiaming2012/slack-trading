package eventmodels

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_TradierOrderDataStore(t *testing.T) {
	t.Run("add an order", func(t *testing.T) {
		// arrange
		orders := NewTradierOrderDataStore()
		order := &TradierOrder{
			ID: 1,
		}

		// act
		orders.Add(order)

		// assert
		assert.Equal(t, 1, len(orders))
		assert.Equal(t, order, orders[order.ID])
	})

	t.Run("delete an order", func(t *testing.T) {
		// arrange
		orders := NewTradierOrderDataStore()
		order := &TradierOrder{
			ID: 1,
		}
		orders.Add(order)

		// act
		orders.Delete(order.ID)

		// assert
		assert.Equal(t, 0, len(orders))
	})

	t.Run("update an order", func(t *testing.T) {
		// arrange
		orders := NewTradierOrderDataStore()
		order := &TradierOrder{
			ID:     1,
			Status: "open",
		}
		orders.Add(order)

		// act
		update := &TradierOrder{
			ID:     1,
			Status: "filled",
		}

		updates := orders.Update(update)

		// assert
		assert.Equal(t, 1, len(updates))
		assert.Equal(t, "status", updates[0].Field)
		assert.Equal(t, "open", updates[0].Old)
		assert.Equal(t, "filled", updates[0].New)
		assert.Equal(t, "filled", orders[order.ID].Status)
	})

	t.Run("update an order that does not exist", func(t *testing.T) {
		// arrange
		orders := NewTradierOrderDataStore()

		// act
		update := &TradierOrder{
			ID:     1,
			Status: "filled",
		}

		updates := orders.Update(update)

		// assert
		assert.Equal(t, 0, len(updates))
	})

	t.Run("fail to update an order with mismatch ID", func(t *testing.T) {
		// arrange
		orders := NewTradierOrderDataStore()
		order := &TradierOrder{
			ID:     1,
			Status: "open",
		}
		orders.Add(order)

		// act
		update := &TradierOrder{
			ID:     2,
			Status: "filled",
		}

		updates := orders.Update(update)

		// assert
		assert.Equal(t, 0, len(updates))
		assert.Equal(t, len(orders), 1)
		assert.Equal(t, "open", orders[order.ID].Status)
	})
}
