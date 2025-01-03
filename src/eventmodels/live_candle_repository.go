package eventmodels

type LiveCandleRepository struct {
	Instrument Instrument
	Period     TradierInterval
}
