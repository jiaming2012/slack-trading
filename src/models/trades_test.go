package models

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestProfit(t *testing.T) {
	t.Run("profitable trades", func(t *testing.T) {
		trades := Trades([]*Trade{
			{
				Volume:        1.0,
				ExecutedPrice: 1000.0,
			},
			{
				Volume:        -0.5,
				ExecutedPrice: 1100.0,
			},
		})

		profit := trades.PL(1300.0)

		assert.Equal(t, 50.0, profit.Realized)
		assert.Equal(t, 150.0, profit.Floating)
	})

	t.Run("losing trades", func(t *testing.T) {
		trades := Trades([]*Trade{
			{
				Volume:        1.0,
				ExecutedPrice: 1000.0,
			},
			{
				Volume:        -0.5,
				ExecutedPrice: 500.0,
			},
		})

		profit := trades.PL(400.0)

		assert.Equal(t, -250.0, profit.Realized)
		assert.Equal(t, -300.0, profit.Floating)

		trades.Add(&Trade{
			Volume:        -0.5,
			ExecutedPrice: 400.0,
		})

		profit = trades.PL(400.0)

		assert.Equal(t, -550.0, profit.Realized)
		assert.Equal(t, 0.0, profit.Floating)
	})

	t.Run("losing -> winning trades", func(t *testing.T) {
		trades := Trades([]*Trade{
			{
				Volume:        1.0,
				ExecutedPrice: 1000.0,
			},
			{
				Volume:        -0.5,
				ExecutedPrice: 500.0,
			},
		})

		profit := trades.PL(400.0)

		assert.Equal(t, -250.0, profit.Realized)
		assert.Equal(t, -300.0, profit.Floating)

		trades.Add(&Trade{
			Volume:        -0.3,
			ExecutedPrice: 5000.0,
		})

		profit = trades.PL(5000.0)

		assert.Equal(t, -250.0+1200.0, profit.Realized)
		assert.Equal(t, 800.0, profit.Floating)
	})

	t.Run("no trades", func(t *testing.T) {
		trades := Trades([]*Trade{})
		profit := trades.PL(1000.0)
		assert.Equal(t, 0.0, profit.Realized)
		assert.Equal(t, 0.0, profit.Floating)
	})

	t.Run("close an open trade", func(t *testing.T) {
		trades := Trades([]*Trade{
			{
				Volume:        1.0,
				ExecutedPrice: 1000.0,
			},
		})

		profit := trades.PL(2000.0)
		assert.Equal(t, 0.0, profit.Realized)
		assert.Equal(t, 1000.0, profit.Floating)

		trades.Add(&Trade{
			Volume:        -1.0,
			ExecutedPrice: 2000.0,
		})

		profit = trades.PL(2000.0)
		assert.Equal(t, 1000.0, profit.Realized)
		assert.Equal(t, 0.0, profit.Floating)
	})

	t.Run("floating profit long", func(t *testing.T) {
		trades := Trades([]*Trade{
			{
				Volume:        1.0,
				ExecutedPrice: 1000.0,
			},
			{
				Volume:        1.0,
				ExecutedPrice: 2000.0,
			},
		})
		profit := trades.PL(3000.0)
		assert.Equal(t, 0.0, profit.Realized)
		assert.Equal(t, 3000.0, profit.Floating)
	})

	t.Run("floating profit short", func(t *testing.T) {
		trades := Trades([]*Trade{
			{
				Volume:        -1.0,
				ExecutedPrice: 1000.0,
			},
			{
				Volume:        -1.0,
				ExecutedPrice: 2000.0,
			},
		})
		profit := trades.PL(3000.0)
		assert.Equal(t, 0.0, profit.Realized)
		assert.Equal(t, -3000.0, profit.Floating)
	})
}

func TestVwap(t *testing.T) {
	t.Run("long and short trades", func(t *testing.T) {
		trades := Trades([]*Trade{
			{
				Volume:        1.0,
				ExecutedPrice: 1000.0,
			},
			{
				Volume:        -0.5,
				ExecutedPrice: 1100.0,
			},
		})

		vwap, volume, realizedPL := trades.Vwap()

		assert.Equal(t, Volume(0.5), volume)
		assert.Equal(t, Vwap(1000.0), vwap)
		assert.Equal(t, RealizedPL(50.0), realizedPL)
	})

	t.Run("no trades", func(t *testing.T) {
		trades := Trades([]*Trade{})

		vwap, volume, realizedPL := trades.Vwap()

		assert.Equal(t, Volume(0.0), volume)
		assert.Equal(t, Vwap(0.0), vwap)
		assert.Equal(t, RealizedPL(0.0), realizedPL)
	})

	t.Run("switch volume direction: long -> short", func(t *testing.T) {
		trades := Trades([]*Trade{
			{
				Volume:        1.0,
				ExecutedPrice: 1000.0,
			},
			{
				Volume:        -0.5,
				ExecutedPrice: 1100.0,
			},
			{
				Volume:        -0.7,
				ExecutedPrice: 1500.0,
			},
		})

		vwap, volume, realizedPL := trades.Vwap()

		assert.LessOrEqual(t, Volume(-0.2)-volume, 0.01)
		assert.Equal(t, Vwap(1500.0), vwap)
		assert.Equal(t, RealizedPL(300.0), realizedPL)
	})

	t.Run("switch volume direction: short -> long", func(t *testing.T) {
		trades := Trades([]*Trade{
			{
				Volume:        -1.0,
				ExecutedPrice: 1000.0,
			},
			{
				Volume:        1.7,
				ExecutedPrice: 1300.0,
			},
			{
				Volume:        -0.5,
				ExecutedPrice: 1100.0,
			},
		})

		vwap, volume, realizedPL := trades.Vwap()

		assert.LessOrEqual(t, Volume(-0.2)-volume, 0.001)
		assert.Equal(t, Vwap(1300.0), vwap)
		assert.Equal(t, RealizedPL(-400.0), realizedPL)
	})
}
