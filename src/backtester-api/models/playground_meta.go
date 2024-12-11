package models

type PlaygroundMeta struct {
	StartDate       string   `json:"start_date"`
	EndDate         string   `json:"end_date"`
	Symbols         []string `json:"symbols"`
	StartingBalance float64  `json:"starting_balance"`
}
