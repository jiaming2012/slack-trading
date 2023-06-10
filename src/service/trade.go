package service

import "slack-trading/src/models"

func PlaceBuy(account *models.Account, currentPrice float64, stopLoss float64) (*models.Trade, error) {
	return account.PlaceOrder(models.TradeTypeBuy, currentPrice, stopLoss, -1)
}

func PlaceSell(account *models.Account, currentPrice float64, stopLoss float64) (*models.Trade, error) {
	return account.PlaceOrder(models.TradeTypeSell, currentPrice, stopLoss, -1)
}

func PlaceClose(account *models.Account, currentPrice float64, closePercent float64) (*models.Trade, error) {
	return account.PlaceOrder(models.TradeTypeBuy, currentPrice, -1, closePercent)
}
