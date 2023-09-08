package service

import "slack-trading/src/models"

func PlaceBuy(account *models.Account, currentPrice float64, stopLoss float64) (*models.Trade, error) {
	return account.placeOrder(models.TradeTypeBuy, currentPrice, stopLoss, -1)
}

func PlaceSell(account *models.Account, currentPrice float64, stopLoss float64) (*models.Trade, error) {
	return account.placeOrder(models.TradeTypeSell, currentPrice, stopLoss, -1)
}

func PlaceClose(account *models.Account, currentPrice float64, closePercent float64) (*models.Trade, error) {
	return account.placeOrder(models.TradeTypeBuy, currentPrice, -1, closePercent)
}
