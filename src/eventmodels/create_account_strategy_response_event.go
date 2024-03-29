package eventmodels

type CreateAccountStrategyResponseEvent struct {
	AccountsRequestHeader
	Strategy *Strategy `json:"strategy"`
}
