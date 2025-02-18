package models

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConvertCandles(t *testing.T) {
	t.Run("error when periods do not divide evenly", func(t *testing.T) {
		candles := Candles{
			Period: 5,
		}

		_, err := candles.ConvertTo(7)

		require.NotEmpty(t, err)
	})

	t.Run("error when convert period is lower than base candles period", func(t *testing.T) {
		candles := Candles{
			Period: 5,
		}

		_, err := candles.ConvertTo(0)

		require.NotEmpty(t, err)
	})

	t.Run("convert even period of candles. new candle size = 1", func(t *testing.T) {
		candles := Candles{
			Period: 5,
			Data: []Candle{
				{
					Open:  2,
					High:  4,
					Low:   1,
					Close: 3,
				},
				{
					Open:  3,
					High:  6,
					Low:   1,
					Close: 9,
				},
				{
					Open:  9,
					High:  4,
					Low:   0.5,
					Close: 8,
				},
			},
		}

		newCandles, err := candles.ConvertTo(15)
		require.NoError(t, err)

		require.Equal(t, 15, newCandles.Period)
		require.Len(t, newCandles.Data, 1)
		require.Equal(t, 2.0, newCandles.Data[0].Open)
		require.Equal(t, 6.0, newCandles.Data[0].High)
		require.Equal(t, 0.5, newCandles.Data[0].Low)
		require.Equal(t, 8.0, newCandles.Data[0].Close)
	})

	t.Run("convert even period of candles. new candle size = 2", func(t *testing.T) {
		candles := Candles{
			Period: 5,
			Data: []Candle{
				{
					Open:  2,
					High:  4,
					Low:   1,
					Close: 3,
				},
				{
					Open:  3,
					High:  6,
					Low:   1,
					Close: 9,
				},
				{
					Open:  9,
					High:  4,
					Low:   0.5,
					Close: 8,
				},
				{
					Open:  8,
					High:  10,
					Low:   1,
					Close: 3,
				},
				{
					Open:  3,
					High:  6,
					Low:   1,
					Close: 2,
				},
				{
					Open:  5,
					High:  4,
					Low:   1,
					Close: 6,
				},
			},
		}

		newCandles, err := candles.ConvertTo(15)
		require.NoError(t, err)

		require.Equal(t, 15, newCandles.Period)
		require.Len(t, newCandles.Data, 2)
		require.Equal(t, 2.0, newCandles.Data[0].Open)
		require.Equal(t, 6.0, newCandles.Data[0].High)
		require.Equal(t, 0.5, newCandles.Data[0].Low)
		require.Equal(t, 8.0, newCandles.Data[0].Close)

		require.Equal(t, 8.0, newCandles.Data[1].Open)
		require.Equal(t, 10.0, newCandles.Data[1].High)
		require.Equal(t, 1.0, newCandles.Data[1].Low)
		require.Equal(t, 6.0, newCandles.Data[1].Close)
	})

	t.Run("convert non-even period of candles", func(t *testing.T) {
		candles := Candles{
			Period: 5,
			Data: []Candle{
				{
					Open:  2,
					High:  4,
					Low:   1,
					Close: 3,
				},
				{
					Open:  3,
					High:  6,
					Low:   1,
					Close: 9,
				},
				{
					Open:  9,
					High:  4,
					Low:   0.5,
					Close: 8,
				},
				{
					Open:  8,
					High:  10,
					Low:   1,
					Close: 3,
				},
				{
					Open:  3,
					High:  6,
					Low:   1,
					Close: 2,
				},
				{
					Open:  5,
					High:  4,
					Low:   1,
					Close: 6,
				},
				{
					Open:  6,
					High:  6,
					Low:   1,
					Close: 2,
				},
				{
					Open:  2,
					High:  3,
					Low:   1,
					Close: 1,
				},
			},
		}

		newCandles, err := candles.ConvertTo(15)
		require.NoError(t, err)

		require.Equal(t, 15, newCandles.Period)
		require.Len(t, newCandles.Data, 3)
		require.Equal(t, 2.0, newCandles.Data[0].Open)
		require.Equal(t, 6.0, newCandles.Data[0].High)
		require.Equal(t, 0.5, newCandles.Data[0].Low)
		require.Equal(t, 8.0, newCandles.Data[0].Close)

		require.Equal(t, 8.0, newCandles.Data[1].Open)
		require.Equal(t, 10.0, newCandles.Data[1].High)
		require.Equal(t, 1.0, newCandles.Data[1].Low)
		require.Equal(t, 6.0, newCandles.Data[1].Close)

		require.Equal(t, 6.0, newCandles.Data[2].Open)
		require.Equal(t, 6.0, newCandles.Data[2].High)
		require.Equal(t, 1.0, newCandles.Data[2].Low)
		require.Equal(t, 1.0, newCandles.Data[2].Close)
	})
}
