package models

type PlaygroundSource struct {
	Broker      string       `json:"broker"`
	AccountID   string       `json:"account_id"`
	AccountType *AccountRole `json:"account_type"`
}
