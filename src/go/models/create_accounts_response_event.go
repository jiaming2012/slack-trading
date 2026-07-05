package models

type CreateAccountResponseEvent struct {
	BaseResponseEvent
	Account *Account `json:"account"`
}
