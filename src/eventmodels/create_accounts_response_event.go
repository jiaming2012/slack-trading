package eventmodels

type CreateAccountResponseEvent struct {
	BaseResponseEvent
	Account *Account `json:"account"`
}
