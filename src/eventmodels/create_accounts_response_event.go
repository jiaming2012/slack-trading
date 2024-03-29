package eventmodels

type CreateAccountResponseEvent struct {
	BaseResponseEvent2
	Account *Account `json:"account"`
}
