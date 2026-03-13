package models

type CreateAccountRequest struct {
	Balance float64                     `json:"balance"`
	Source  *CreateAccountRequestSource `json:"source"`
}
