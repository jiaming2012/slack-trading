package eventmodels

type AccountsRequestHeader struct {
	BaseRequestEvent
	AccountName string `json:"accountName"`
}
