package models

type CreateAccountRequestSource struct {
	Broker      string          `json:"broker"`
	AccountID   string          `json:"account_id"`
	AccountType LiveAccountType `json:"account_type"`
}
