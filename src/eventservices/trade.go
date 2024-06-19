package eventservices

import "github.com/jiaming2012/slack-trading/src/models"

func RealizedDrawdown(trade *models.Trade, candles []*models.Candle, state map[string]interface{}) float64 {
	maxDrawdownPrice := 0.0
	if trade.Type == models.TradeTypeBuy {
		for _, t := range candles {
			if trade.Timestamp.After(t.Timestamp) {
				continue
			}

			if maxDrawdownPrice <= 0.0 || t.Low < maxDrawdownPrice {
				maxDrawdownPrice = t.Low
			}
		}
	} else if trade.Type == models.TradeTypeSell {
		for _, t := range candles {
			if trade.Timestamp.After(t.Timestamp) {
				continue
			}

			if maxDrawdownPrice <= 0.0 || t.High > maxDrawdownPrice {
				maxDrawdownPrice = t.High
			}
		}
	}

	return maxDrawdownPrice
}
