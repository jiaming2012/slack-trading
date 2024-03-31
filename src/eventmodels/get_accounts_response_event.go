package eventmodels

type GetAccountsResponseEvent struct {
	BaseResponseEvent
	Accounts []*Account `json:"accounts"`
}
