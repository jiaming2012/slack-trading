package models

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestProfit(t *testing.T) {
	t.Run("profitable trades", func(t *testing.T) {
		trades := Trades([]Trade{
			{
				Volume: 1.0,
				Price:  1000.0,
			},
			{
				Volume: -0.5,
				Price:  1100.0,
			},
		})

		profit := trades.PL(1300.0)

		assert.Equal(t, 50.0, profit.Realized)
		assert.Equal(t, 150.0, profit.Floating)
	})

	t.Run("losing trades", func(t *testing.T) {
		trades := Trades([]Trade{
			{
				Volume: 1.0,
				Price:  1000.0,
			},
			{
				Volume: -0.5,
				Price:  500.0,
			},
		})

		profit := trades.PL(400.0)

		assert.Equal(t, -250.0, profit.Realized)
		assert.Equal(t, -300.0, profit.Floating)

		trades.Add(&Trade{
			Volume: -0.5,
			Price:  400.0,
		})

		profit = trades.PL(400.0)

		assert.Equal(t, -550.0, profit.Realized)
		assert.Equal(t, 0.0, profit.Floating)
	})

	t.Run("losing -> winning trades", func(t *testing.T) {
		trades := Trades([]Trade{
			{
				Volume: 1.0,
				Price:  1000.0,
			},
			{
				Volume: -0.5,
				Price:  500.0,
			},
		})

		profit := trades.PL(400.0)

		assert.Equal(t, -250.0, profit.Realized)
		assert.Equal(t, -300.0, profit.Floating)

		trades.Add(&Trade{
			Volume: -0.3,
			Price:  5000.0,
		})

		profit = trades.PL(5000.0)

		assert.Equal(t, -250.0+1200.0, profit.Realized)
		assert.Equal(t, 800.0, profit.Floating)
	})

	t.Run("no trades", func(t *testing.T) {
		trades := Trades([]Trade{})
		profit := trades.PL(1000.0)
		assert.Equal(t, 0.0, profit.Realized)
		assert.Equal(t, 0.0, profit.Floating)
	})

	t.Run("floating profit long", func(t *testing.T) {
		trades := Trades([]Trade{
			{
				Volume: 1.0,
				Price:  1000.0,
			},
			{
				Volume: 1.0,
				Price:  2000.0,
			},
		})
		profit := trades.PL(3000.0)
		assert.Equal(t, 0.0, profit.Realized)
		assert.Equal(t, 3000.0, profit.Floating)
	})

	t.Run("floating profit short", func(t *testing.T) {
		trades := Trades([]Trade{
			{
				Volume: -1.0,
				Price:  1000.0,
			},
			{
				Volume: -1.0,
				Price:  2000.0,
			},
		})
		profit := trades.PL(3000.0)
		assert.Equal(t, 0.0, profit.Realized)
		assert.Equal(t, -3000.0, profit.Floating)
	})
}

func TestVwap(t *testing.T) {
	t.Run("long and short trades", func(t *testing.T) {
		trades := Trades([]Trade{
			{
				Volume: 1.0,
				Price:  1000.0,
			},
			{
				Volume: -0.5,
				Price:  1100.0,
			},
		})

		assert.Equal(t, 1000.0, trades.Vwap())
	})

	t.Run("no trades", func(t *testing.T) {
		trades := Trades([]Trade{})
		assert.Equal(t, 0.0, trades.Vwap())
	})

	t.Run("switch volume direction: long -> short", func(t *testing.T) {
		trades := Trades([]Trade{
			{
				Volume: 1.0,
				Price:  1000.0,
			},
			{
				Volume: -0.5,
				Price:  1100.0,
			},
			{
				Volume: -0.7,
				Price:  1200.0,
			},
		})

		assert.Equal(t, 1200.0, trades.Vwap())
	})

	t.Run("switch volume direction: short -> long", func(t *testing.T) {
		trades := Trades([]Trade{
			{
				Volume: -1.0,
				Price:  1000.0,
			},
			{
				Volume: 1.7,
				Price:  1300.0,
			},
			{
				Volume: -0.5,
				Price:  1100.0,
			},
		})

		assert.Equal(t, 1300.0, trades.Vwap())
	})
}
