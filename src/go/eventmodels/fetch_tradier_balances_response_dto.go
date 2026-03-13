package eventmodels

type FetchTradierBalancesResponseDTO struct {
	Balances struct {
		TotalEquity float64 `json:"total_equity"`
		AccountType string  `json:"account_type"`
		OpenPL      float64 `json:"open_pl"`
		ClosePL     float64 `json:"close_pl"`
	} `json:"balances"`
}
