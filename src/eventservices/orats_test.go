package eventservices

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func TestConvertOratsOptionDataToCandlesDTO__Calls(t *testing.T) {
	data := []eventmodels.OratsOptionData{
		{
			Ticker:          "IWM",
			SnapShotEstTime: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
			CallBidPrice:    1.0,
		},
		{
			Ticker:          "IWM",
			SnapShotEstTime: time.Date(2021, 1, 1, 0, 1, 0, 0, time.UTC),
			CallBidPrice:    2.0,
		},
		{
			Ticker:          "IWM",
			SnapShotEstTime: time.Date(2021, 1, 1, 0, 14, 0, 0, time.UTC),
			CallBidPrice:    6.0,
		},
		{
			Ticker:          "IWM",
			SnapShotEstTime: time.Date(2021, 1, 1, 0, 15, 0, 0, time.UTC),
			CallBidPrice:    5.9,
		},
		{
			Ticker:          "IWM",
			SnapShotEstTime: time.Date(2021, 1, 1, 0, 28, 0, 0, time.UTC),
			CallBidPrice:    2.1,
		},
		{
			Ticker:          "IWM",
			SnapShotEstTime: time.Date(2021, 1, 1, 0, 29, 0, 0, time.UTC),
			CallBidPrice:    3.3,
		},
		{
			Ticker:          "IWM",
			SnapShotEstTime: time.Date(2021, 1, 1, 0, 30, 0, 0, time.UTC),
			CallBidPrice:    4.0,
		},
	}

	t.Run("data is empty", func(t *testing.T) {
		data := []eventmodels.OratsOptionData{}
		candlesDTO, err := ConvertOratsOptionDataToCandlesDTO(data, 1*time.Minute, eventmodels.OptionTypeCall)
		assert.NoError(t, err)
		assert.Len(t, candlesDTO, 0)
	})

	t.Run("convert call options", func(t *testing.T) {
		candlesDTO, err := ConvertOratsOptionDataToCandlesDTO(data, 15*time.Minute, eventmodels.OptionTypeCall)
		assert.NoError(t, err)
		assert.Len(t, candlesDTO, 3)

		assert.Equal(t, 1.0, candlesDTO[0].Open)
		assert.Equal(t, 6.0, candlesDTO[0].Close)
		assert.Equal(t, 6.0, candlesDTO[0].High)
		assert.Equal(t, 1.0, candlesDTO[0].Low)

		assert.Equal(t, 5.9, candlesDTO[1].Open)
		assert.Equal(t, 3.3, candlesDTO[1].Close)
		assert.Equal(t, 5.9, candlesDTO[1].High)
		assert.Equal(t, 2.1, candlesDTO[1].Low)

		assert.Equal(t, 4.0, candlesDTO[2].Open)
		assert.Equal(t, 4.0, candlesDTO[2].Close)
		assert.Equal(t, 4.0, candlesDTO[2].High)
		assert.Equal(t, 4.0, candlesDTO[2].Low)
	})
}

func TestConvertOratsOptionDataToCandlesDTO__Puts(t *testing.T) {
	data := []eventmodels.OratsOptionData{
		{
			Ticker:          "IWM",
			SnapShotEstTime: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
			PutBidPrice:     11.0,
		},
		{
			Ticker:          "IWM",
			SnapShotEstTime: time.Date(2021, 1, 1, 0, 1, 0, 0, time.UTC),
			PutBidPrice:     12.0,
		},
		{
			Ticker:          "IWM",
			SnapShotEstTime: time.Date(2021, 1, 1, 0, 14, 0, 0, time.UTC),
			PutBidPrice:     16.0,
		},
		{
			Ticker:          "IWM",
			SnapShotEstTime: time.Date(2021, 1, 1, 0, 15, 0, 0, time.UTC),
			PutBidPrice:     15.9,
		},
		{
			Ticker:          "IWM",
			SnapShotEstTime: time.Date(2021, 1, 1, 0, 28, 0, 0, time.UTC),
			PutBidPrice:     12.1,
		},
		{
			Ticker:          "IWM",
			SnapShotEstTime: time.Date(2021, 1, 1, 0, 29, 0, 0, time.UTC),
			PutBidPrice:     13.3,
		},
		{
			Ticker:          "IWM",
			SnapShotEstTime: time.Date(2021, 1, 1, 0, 30, 0, 0, time.UTC),
			PutBidPrice:     14.0,
		},
	}

	t.Run("data is empty", func(t *testing.T) {
		data := []eventmodels.OratsOptionData{}
		candlesDTO, err := ConvertOratsOptionDataToCandlesDTO(data, 1*time.Minute, eventmodels.OptionTypePut)
		assert.NoError(t, err)
		assert.Len(t, candlesDTO, 0)
	})

	t.Run("convert call options", func(t *testing.T) {
		candlesDTO, err := ConvertOratsOptionDataToCandlesDTO(data, 15*time.Minute, eventmodels.OptionTypePut)
		assert.NoError(t, err)
		assert.Len(t, candlesDTO, 3)

		assert.Equal(t, 11.0, candlesDTO[0].Open)
		assert.Equal(t, 16.0, candlesDTO[0].Close)
		assert.Equal(t, 16.0, candlesDTO[0].High)
		assert.Equal(t, 11.0, candlesDTO[0].Low)

		assert.Equal(t, 15.9, candlesDTO[1].Open)
		assert.Equal(t, 13.3, candlesDTO[1].Close)
		assert.Equal(t, 15.9, candlesDTO[1].High)
		assert.Equal(t, 12.1, candlesDTO[1].Low)

		assert.Equal(t, 14.0, candlesDTO[2].Open)
		assert.Equal(t, 14.0, candlesDTO[2].Close)
		assert.Equal(t, 14.0, candlesDTO[2].High)
		assert.Equal(t, 14.0, candlesDTO[2].Low)
	})
}
