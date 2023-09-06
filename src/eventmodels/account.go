package eventmodels

import "slack-trading/src/models"

type GetAccountsRequestEvent struct{}

type GetAccountsResponseEvent struct {
	Accounts []models.Account
}

type AddAccountRequestEvent struct {
	Name              string
	Balance           float64
	MaxLossPercentage float64
	PriceLevelsInput  [][3]float64
}

type AddAccountResponseEvent struct {
	Account models.Account
}
