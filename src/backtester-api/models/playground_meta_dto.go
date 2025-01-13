package models

type PlaygroundMetaDTO struct {
	StartDate       string   `json:"start_date"`
	EndDate         string   `json:"end_date"`
	Symbols         []string `json:"symbols"`
	StartingBalance float64  `json:"starting_balance"`
	Environment     string   `json:"environment"`
	SourceBroker    string   `json:"source_broker"`
	SourceAccountId string   `json:"source_account_id"`
	SourceApiKey    string   `json:"source_api"`
}
