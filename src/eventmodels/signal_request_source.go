package eventmodels

type SignalSource string

const (
	SignalSourceTradingView SignalSource = "TradingView"
	SignalSourceTrendSpider SignalSource = "TrendSpider"
	SignalSourceWebClient   SignalSource = "WebClient"
	SignalSourceManual      SignalSource = "Manual"
)
