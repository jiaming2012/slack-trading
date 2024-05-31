package eventmodels

type CsvTradingViewCandleDTO struct {
	Timestamp       string `csv:"time"`
	Open            string `csv:"open"`
	High            string `csv:"high"`
	Low             string `csv:"low"`
	Close           string `csv:"close"`
	UpTrend         string `csv:"Up Trend"`
	UpTrendBegins   string `csv:"UpTrend Begins"`
	DownTrend       string `csv:"Down Trend"`
	DownTrendBegins string `csv:"DownTrend Begins"`
	K               string `csv:"K"`
	D               string `csv:"D"`
}
