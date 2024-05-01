package eventmodels

type SignalRequestSource string

const (
	TradingView SignalRequestSource = "TradingView"
	TrendSpider SignalRequestSource = "TrendSpider"
	WebClient   SignalRequestSource = "WebClient"
)
