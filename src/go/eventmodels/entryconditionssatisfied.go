package eventmodels

type EntryConditionsSatisfied struct {
	Account  *Account
	Strategy *Strategy
}

func NewEntryConditionsSatisfied(account *Account, strategy *Strategy) *EntryConditionsSatisfied {
	return &EntryConditionsSatisfied{Account: account, Strategy: strategy}
}
