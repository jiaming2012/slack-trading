package utils

import (
	"testing"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/stretchr/testify/require"
)

const noPostionsJSONResponse = `
{
    "positions": "null"
}
`

const singlePositionJSONResponse = `
{
    "positions": {
        "position": {
            "cost_basis": 695.00,
            "date_acquired": "2024-08-12T14:56:31.371Z",
            "id": 995716,
            "quantity": 1.00000000,
            "symbol": "COIN240816C00197500"
        }
    }
}
`

const multiplePositionsJSONResponse = `
{
    "positions": {
        "position": [
            {
                "cost_basis": 373.00,
                "date_acquired": "2024-08-12T14:33:40.267Z",
                "id": 995566,
                "quantity": 1.00000000,
                "symbol": "IWM240816P00207000"
            },
            {
                "cost_basis": -435.00,
                "date_acquired": "2024-08-12T14:33:40.267Z",
                "id": 995567,
                "quantity": -1.00000000,
                "symbol": "IWM240816P00208000"
            },
            {
                "cost_basis": 266.00,
                "date_acquired": "2024-08-12T18:30:25.051Z",
                "id": 996427,
                "quantity": 1.00000000,
                "symbol": "NVDA240816P00109000"
            },
            {
                "cost_basis": -2010.00,
                "date_acquired": "2024-08-12T18:30:25.051Z",
                "id": 996428,
                "quantity": -1.00000000,
                "symbol": "NVDA240816P00130000"
            },
            {
                "cost_basis": -1042.00,
                "date_acquired": "2024-08-12T20:05:29.359Z",
                "id": 996756,
                "quantity": -2.00000000,
                "symbol": "QQQ240813C00447000"
            },
            {
                "cost_basis": 412.00,
                "date_acquired": "2024-08-12T20:05:29.359Z",
                "id": 996755,
                "quantity": 2.00000000,
                "symbol": "QQQ240813C00452000"
            },
            {
                "cost_basis": 778.00,
                "date_acquired": "2024-08-12T20:05:35.202Z",
                "id": 996759,
                "quantity": 2.00000000,
                "symbol": "QQQ240813P00453000"
            },
            {
                "cost_basis": -928.00,
                "date_acquired": "2024-08-12T20:05:35.202Z",
                "id": 996760,
                "quantity": -2.00000000,
                "symbol": "QQQ240813P00454000"
            },
            {
                "cost_basis": -441.00,
                "date_acquired": "2024-08-12T20:15:12.870Z",
                "id": 996779,
                "quantity": -1.00000000,
                "symbol": "QQQ240815C00453000"
            },
            {
                "cost_basis": 366.00,
                "date_acquired": "2024-08-12T20:15:12.870Z",
                "id": 996778,
                "quantity": 1.00000000,
                "symbol": "QQQ240815C00454000"
            },
            {
                "cost_basis": 836.00,
                "date_acquired": "2024-08-12T20:09:07.176Z",
                "id": 996765,
                "quantity": 2.00000000,
                "symbol": "QQQ240815P00448000"
            },
            {
                "cost_basis": -932.00,
                "date_acquired": "2024-08-12T20:09:07.176Z",
                "id": 996766,
                "quantity": -2.00000000,
                "symbol": "QQQ240815P00449000"
            },
            {
                "cost_basis": -1646.00,
                "date_acquired": "2024-08-12T20:08:24.201Z",
                "id": 996764,
                "quantity": -2.00000000,
                "symbol": "QQQ240816C00447000"
            },
            {
                "cost_basis": -743.00,
                "date_acquired": "2024-08-12T20:15:12.041Z",
                "id": 996775,
                "quantity": -1.00000000,
                "symbol": "QQQ240816C00449000"
            },
            {
                "cost_basis": 546.00,
                "date_acquired": "2024-08-12T20:15:12.041Z",
                "id": 996774,
                "quantity": 1.00000000,
                "symbol": "QQQ240816C00452000"
            },
            {
                "cost_basis": 922.00,
                "date_acquired": "2024-08-12T20:08:24.201Z",
                "id": 996763,
                "quantity": 2.00000000,
                "symbol": "QQQ240816C00453000"
            },
            {
                "cost_basis": 862.00,
                "date_acquired": "2024-08-12T20:10:25.225Z",
                "id": 996767,
                "quantity": 2.00000000,
                "symbol": "QQQ240816P00447000"
            },
            {
                "cost_basis": 1108.00,
                "date_acquired": "2024-08-12T20:01:50.084Z",
                "id": 996735,
                "quantity": 2.00000000,
                "symbol": "QQQ240816P00450000"
            },
            {
                "cost_basis": -2430.00,
                "date_acquired": "2024-08-12T20:01:50.084Z",
                "id": 996736,
                "quantity": -4.00000000,
                "symbol": "QQQ240816P00451000"
            },
            {
                "cost_basis": -690.00,
                "date_acquired": "2024-08-12T19:28:52.317Z",
                "id": 996575,
                "quantity": -1.00000000,
                "symbol": "SPY240813C00527000"
            },
            {
                "cost_basis": 298.00,
                "date_acquired": "2024-08-12T19:28:52.317Z",
                "id": 996574,
                "quantity": 1.00000000,
                "symbol": "SPY240813C00532000"
            },
            {
                "cost_basis": 18.00,
                "date_acquired": "2024-08-12T18:46:01.987Z",
                "id": 996471,
                "quantity": 2.00000000,
                "symbol": "SPY240813P00515000"
            },
            {
                "cost_basis": -294.00,
                "date_acquired": "2024-08-12T18:46:01.987Z",
                "id": 996472,
                "quantity": -1.00000000,
                "symbol": "SPY240813P00533000"
            },
            {
                "cost_basis": -1014.00,
                "date_acquired": "2024-08-12T18:46:28.025Z",
                "id": 996475,
                "quantity": -1.00000000,
                "symbol": "SPY240813P00543000"
            },
            {
                "cost_basis": 157.00,
                "date_acquired": "2024-08-12T18:46:01.472Z",
                "id": 996469,
                "quantity": 2.00000000,
                "symbol": "SPY240815P00515000"
            },
            {
                "cost_basis": -3074.00,
                "date_acquired": "2024-08-12T18:46:01.472Z",
                "id": 996470,
                "quantity": -2.00000000,
                "symbol": "SPY240815P00548000"
            },
            {
                "cost_basis": -4907.00,
                "date_acquired": "2024-08-12T14:33:40.923Z",
                "id": 995569,
                "quantity": -2.00000000,
                "symbol": "SPY240816C00511000"
            },
            {
                "cost_basis": 1595.00,
                "date_acquired": "2024-08-12T15:17:39.898Z",
                "id": 995844,
                "quantity": 1.00000000,
                "symbol": "SPY240816C00521000"
            },
            {
                "cost_basis": -2690.00,
                "date_acquired": "2024-08-12T20:15:12.419Z",
                "id": 996777,
                "quantity": -2.00000000,
                "symbol": "SPY240816C00522000"
            },
            {
                "cost_basis": 1864.00,
                "date_acquired": "2024-08-12T20:15:12.419Z",
                "id": 996776,
                "quantity": 2.00000000,
                "symbol": "SPY240816C00527000"
            },
            {
                "cost_basis": 684.00,
                "date_acquired": "2024-08-12T14:33:40.923Z",
                "id": 995568,
                "quantity": 1.00000000,
                "symbol": "SPY240816C00531000"
            },
            {
                "cost_basis": 66.00,
                "date_acquired": "2024-08-12T14:15:19.167Z",
                "id": 995408,
                "quantity": 2.00000000,
                "symbol": "SPY240816P00501000"
            },
            {
                "cost_basis": 256.00,
                "date_acquired": "2024-08-12T18:46:27.885Z",
                "id": 996473,
                "quantity": 2.00000000,
                "symbol": "SPY240816P00517000"
            },
            {
                "cost_basis": -2291.00,
                "date_acquired": "2024-08-12T14:15:19.167Z",
                "id": 995409,
                "quantity": -2.00000000,
                "symbol": "SPY240816P00542000"
            },
            {
                "cost_basis": -1117.00,
                "date_acquired": "2024-08-12T19:31:14.295Z",
                "id": 996585,
                "quantity": -1.00000000,
                "symbol": "SPY240816P00543000"
            },
            {
                "cost_basis": -1549.00,
                "date_acquired": "2024-08-12T18:46:27.885Z",
                "id": 996474,
                "quantity": -1.00000000,
                "symbol": "SPY240816P00548000"
            },
            {
                "cost_basis": -6785.00,
                "date_acquired": "2024-08-12T18:45:19.064Z",
                "id": 996467,
                "quantity": -2.00000000,
                "symbol": "TSLA240816C00165000"
            },
            {
                "cost_basis": 430.00,
                "date_acquired": "2024-08-12T18:45:28.660Z",
                "id": 996468,
                "quantity": 1.00000000,
                "symbol": "TSLA240816C00200000"
            },
            {
                "cost_basis": 112.00,
                "date_acquired": "2024-08-12T18:45:19.064Z",
                "id": 996466,
                "quantity": 1.00000000,
                "symbol": "TSLA240816C00210000"
            },
            {
                "cost_basis": 570.00,
                "date_acquired": "2024-08-12T18:48:03.083Z",
                "id": 996479,
                "quantity": 1.00000000,
                "symbol": "TSLA240816P00200000"
            },
            {
                "cost_basis": 1270.00,
                "date_acquired": "2024-08-12T18:54:30.242Z",
                "id": 996497,
                "quantity": 1.00000000,
                "symbol": "TSLA240816P00210000"
            },
            {
                "cost_basis": -6365.00,
                "date_acquired": "2024-08-12T18:48:03.083Z",
                "id": 996480,
                "quantity": -2.00000000,
                "symbol": "TSLA240816P00230000"
            }
        ]
    }
}
`

func TestParseTradierResponse(t *testing.T) {
	t.Run("no positions", func(t *testing.T) {
		dto, err := ParseTradierResponse[eventmodels.TradierPositionDTO]([]byte(noPostionsJSONResponse))

		require.NoError(t, err)

		require.Len(t, dto, 0)
	})

	t.Run("single position", func(t *testing.T) {
		dto, err := ParseTradierResponse[eventmodels.TradierPositionDTO]([]byte(singlePositionJSONResponse))

		require.NoError(t, err)

		require.Len(t, dto, 1)

		require.Equal(t, 695.00, dto[0].CostBasis)
		require.Equal(t, "2024-08-12T14:56:31.371Z", dto[0].DateAcquired)
		require.Equal(t, 995716, dto[0].ID)
		require.Equal(t, 1.00000000, dto[0].Quantity)
		require.Equal(t, "COIN240816C00197500", dto[0].Symbol)
	})

	t.Run("multiple positions", func(t *testing.T) {
		dto, err := ParseTradierResponse[eventmodels.TradierPositionDTO]([]byte(multiplePositionsJSONResponse))

		require.NoError(t, err)

		require.Len(t, dto, 42)

		require.Equal(t, -6365.00, dto[41].CostBasis)
		require.Equal(t, "2024-08-12T18:48:03.083Z", dto[41].DateAcquired)
		require.Equal(t, 996480, dto[41].ID)
		require.Equal(t, -2.00000000, dto[41].Quantity)
		require.Equal(t, "TSLA240816P00230000", dto[41].Symbol)
	})
}

func TestCreateTag(t *testing.T) {
	t.Run("Encode Tag", func(t *testing.T) {
		signal := eventmodels.SignalName("supertrend-4h-1h_stoch_rsi_15m_up")
		tag := EncodeTag(signal, 9.53, 21.45)
		require.Equal(t, tag, "supertrend--4h--1h-stoch-rsi-15m-up---9-53---21-45")
	})

	t.Run("Decode tag", func(t *testing.T) {
		tag := "supertrend--4h--1h-stoch-rsi-15m-up---9-53---21-45"
		signal, expectedProfit, requestedPrc, err := DecodeTag(tag)
		require.NoError(t, err)
		require.Equal(t, eventmodels.SignalName("supertrend-4h-1h_stoch_rsi_15m_up"), signal)
		require.Equal(t, 9.53, expectedProfit)
		require.Equal(t, 21.45, requestedPrc)
	})
}
