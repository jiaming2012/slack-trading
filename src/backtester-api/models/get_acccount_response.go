package models

type GetAccountResponse struct {
	Meta       Meta                 `json:"meta"`
	Balance    float64              `json:"balance"`
	Equity     float64              `json:"equity"`
	FreeMargin float64              `json:"free_margin"`
	Positions  map[string]*Position `json:"positions"`
	Orders     []*OrderRecord       `json:"orders"`
}
