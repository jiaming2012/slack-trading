package models

type CreateAccountStrategyResponseEvent struct {
	AccountsRequestHeader
	Strategy *Strategy `json:"strategy"`
}
