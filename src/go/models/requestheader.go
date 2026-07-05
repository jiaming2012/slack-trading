package models

type AccountsRequestHeader struct {
	BaseRequestEvent
	AccountName string `json:"accountName"`
}
