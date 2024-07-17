package eventmodels

type ThetaDataOptionContract struct {
	Root       StockSymbol         `json:"root"`
	Expiration int                 `json:"expiration"`
	Strike     float64             `json:"strike"`
	Right      ThetaDataOptionType `json:"right"`
}
