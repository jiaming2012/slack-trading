package eventmodels

type CsvTradingViewCandleDTO struct {
	Timestamp       string `csv:"Timestamp"`
	Open            string `csv:"Open"`
	High            string `csv:"High"`
	Low             string `csv:"Low"`
	Close           string `csv:"Close"`
	UpTrend         string `csv:"UpTrend"`
	UpTrendBegins   string `csv:"UpTrendBegins"`
	DownTrend       string `csv:"DownTrend"`
	DownTrendBegins string `csv:"DownTrendBegins"`
	K               string `csv:"K"`
	D               string `csv:"D"`
}
