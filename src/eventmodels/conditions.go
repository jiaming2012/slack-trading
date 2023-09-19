package eventmodels

import "slack-trading/src/models"

type EntryConditionsSatisfied struct {
	Account  *models.Account
	Strategy *models.Strategy
}

func NewEntryConditionsSatisfied(account *models.Account, strategy *models.Strategy) *EntryConditionsSatisfied {
	return &EntryConditionsSatisfied{Account: account, Strategy: strategy}
}
