package eventmodels

type AccountsRequestHeader struct {
	BaseRequestEvent2
	AccountName string `json:"accountName"`
}
