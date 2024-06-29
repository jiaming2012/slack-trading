package eventmodels

import (
	"time"
)

type OratsOptionData struct {
	Ticker           string    `csv:"ticker"`
	TradeDate        string    `csv:"tradeDate"`
	ExpirDate        string    `csv:"expirDate"`
	Dte              int       `csv:"dte"`
	Strike           float64   `csv:"strike"`
	StockPrice       float64   `csv:"stockPrice"`
	CallVolume       int       `csv:"callVolume"`
	CallOpenInterest int       `csv:"callOpenInterest"`
	CallBidSize      int       `csv:"callBidSize"`
	CallAskSize      int       `csv:"callAskSize"`
	PutVolume        int       `csv:"putVolume"`
	PutOpenInterest  int       `csv:"putOpenInterest"`
	PutBidSize       int       `csv:"putBidSize"`
	PutAskSize       int       `csv:"putAskSize"`
	CallBidPrice     float64   `csv:"callBidPrice"`
	CallValue        float64   `csv:"callValue"`
	CallAskPrice     float64   `csv:"callAskPrice"`
	PutBidPrice      float64   `csv:"putBidPrice"`
	PutValue         float64   `csv:"putValue"`
	PutAskPrice      float64   `csv:"putAskPrice"`
	CallBidIv        float64   `csv:"callBidIv"`
	CallMidIv        float64   `csv:"callMidIv"`
	CallAskIv        float64   `csv:"callAskIv"`
	SmvVol           float64   `csv:"smvVol"`
	PutBidIv         float64   `csv:"putBidIv"`
	PutMidIv         float64   `csv:"putMidIv"`
	PutAskIv         float64   `csv:"putAskIv"`
	ResidualRate     float64   `csv:"residualRate"`
	Delta            float64   `csv:"delta"`
	Gamma            float64   `csv:"gamma"`
	Theta            float64   `csv:"theta"`
	Vega             float64   `csv:"vega"`
	Rho              float64   `csv:"rho"`
	Phi              float64   `csv:"phi"`
	DriftlessTheta   float64   `csv:"driftlessTheta"`
	CallSmvVol       float64   `csv:"callSmvVol"`
	PutSmvVol        float64   `csv:"putSmvVol"`
	ExtSmvVol        float64   `csv:"extSmvVol"`
	ExtCallValue     float64   `csv:"extCallValue"`
	ExtPutValue      float64   `csv:"extPutValue"`
	SpotPrice        float64   `csv:"spotPrice"`
	QuoteDate        time.Time `csv:"quoteDate"`
	UpdatedAt        time.Time `csv:"updatedAt"`
	SnapShotEstTime  time.Time `csv:"snapShotEstTime"`
	SnapShotDate     time.Time `csv:"snapShotDate"`
	ExpiryTod        string    `csv:"expiryTod"`
	TickerId         int       `csv:"tickerId"`
	MonthId          int       `csv:"monthId"`
}
