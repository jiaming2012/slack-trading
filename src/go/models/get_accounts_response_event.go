package models

type GetAccountsResponseEvent struct {
	BaseResponseEvent
	Accounts []*Account `json:"accounts"`
}
