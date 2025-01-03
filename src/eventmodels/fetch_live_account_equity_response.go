package eventmodels

type FetchAccountEquityResponse struct {
	Equity  float64 `json:"equity"`
	OpenPL  float64 `json:"open_pl"`
	ClosePL float64 `json:"close_pl"`
}
