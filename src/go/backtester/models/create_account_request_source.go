package models

type CreateAccountRequestSource struct {
	Broker      string      `json:"broker"`
	AccountID   string      `json:"account_id"`
	AccountRole AccountRole `json:"account_type"`
}
