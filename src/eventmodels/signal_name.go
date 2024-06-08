package eventmodels

type SignalName string

const (
	SuperTrendBuy               SignalName = "supertrend-buy"
	SuperTrendSell              SignalName = "supertrend-sell"
	StochasticRsiSell           SignalName = "stochastic_rsi-sell"
	SuperTrend4h1hStochRsi15mUp SignalName = "supertrend-4h-1h_stoch_rsi_15m_up"
)
