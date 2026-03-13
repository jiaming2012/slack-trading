package eventmodels

type TradierCandleUpdate struct {
	Instrument Instrument
	Interval   TradierInterval
	Candle *TradierMarketsTimeSalesDTO
}