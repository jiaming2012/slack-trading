package eventmodels

type TradingViewCandlesDTO []*TradingViewCandleDTO

func (candles TradingViewCandlesDTO) ToModel() []*TradingViewCandle {
	modelCandles := make([]*TradingViewCandle, 0)
	for _, candle := range candles {
		modelCandles = append(modelCandles, candle.ToModel())
	}
	return modelCandles
}
