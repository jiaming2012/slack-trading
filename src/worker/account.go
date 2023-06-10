package worker

import "slack-trading/src/models"

// move to models
type AccountWorker struct {
	account *models.Account
}

func NewAccountWorker(account *models.Account) *AccountWorker {
	return &AccountWorker{
		account: account,
	}
}
