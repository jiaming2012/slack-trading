package models

type EntryConditionsSatisfied struct {
	Account  *Account
	Strategy *Strategy
}

func NewEntryConditionsSatisfied(account *Account, strategy *Strategy) *EntryConditionsSatisfied {
	return &EntryConditionsSatisfied{Account: account, Strategy: strategy}
}

type ExitConditionsSatisfied struct {
	PriceLevel      *PriceLevel
	PriceLevelIndex int
	PercentClose    ClosePercent
	Reason          string
}
