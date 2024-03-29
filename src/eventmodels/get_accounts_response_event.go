package eventmodels

type GetAccountsResponseEvent struct {
	BaseResponseEvent2
	Accounts []*Account `json:"accounts"`
}
