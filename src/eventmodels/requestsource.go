package eventmodels

type RequestSource string

const (
	TradingView RequestSource = "TradingView"
	TrendSpider RequestSource = "TrendSpider"
	WebClient   RequestSource = "WebClient"
)
