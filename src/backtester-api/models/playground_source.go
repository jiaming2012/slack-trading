package models

type PlaygroundSource struct {
	Broker     string `json:"broker"`
	ApiKeyName string `json:"api_key_name"`
	AccountID  string `json:"account_id"`
}
