package eventmodels

type DatafeedName string

const (
	CoinbaseDatafeed DatafeedName = "CoinbaseDatafeed"
	IBDatafeed       DatafeedName = "IBDatafeed"
	ManualDatafeed   DatafeedName = "ManualDatafeed"
)
