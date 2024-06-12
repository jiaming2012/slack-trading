package eventmodels

import (
	log "github.com/sirupsen/logrus"
)

type TradingViewCandlesDTO []*TradingViewCandleDTO

func (candles TradingViewCandlesDTO) ToModel() []*TradingViewCandle {
	modelCandles := make([]*TradingViewCandle, 0)
	for _, candle := range candles {
		c, err := candle.ToModel()
		if err != nil {
			log.Errorf("TradingViewCandlesDTO: error converting to model: %v", err)
			continue
		}

		modelCandles = append(modelCandles, c)
	}
	return modelCandles
}
