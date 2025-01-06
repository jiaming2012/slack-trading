package services

import (
	"context"
	"fmt"
	"os"

	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
	"github.com/jiaming2012/slack-trading/src/utils"
)

type TradierBroker struct {
	url  string
	token string
}

func (b *TradierBroker) PlaceOrder(ctx context.Context, req *models.PlaceEquityTradeRequest) error {
	
	if err := PlaceOrder(ctx, b.url, b.token, req); err != nil {
		return fmt.Errorf("failed to place order: %w", err)
	}
	
	return nil
}

func NewTradierBroker(url, token string) *TradierBroker {
	return &TradierBroker{
		url: url,
		token: token,
	}
}

func CreateLiveAccount(balance float64, accountID, brokerName, apiKeyName string) (*models.LiveAccount, error) {
	if balance < 0 {
		return nil, fmt.Errorf("balance cannot be negative")
	}

	apiKey := os.Getenv(apiKeyName)
	if apiKey == "" {
		return nil, fmt.Errorf("cannot find apiKeyName: %s", apiKeyName)
	}

	tradierBalancesUrlTemplate, err := utils.GetEnv("TRADIER_BALANCES_URL_TEMPLATE")
	if err != nil {
		return nil, fmt.Errorf("$TRADIER_BALANCES_URL_TEMPLATE not set: %v", err)
	}

	url := fmt.Sprintf(tradierBalancesUrlTemplate, accountID)

	source := LiveAccountSource{
		Broker:    brokerName,
		AccountID: accountID,
		ApiKey:    apiKey,
		Url:       url,
	}

	if err := source.Validate(); err != nil {
		return nil, fmt.Errorf("invalid source: %w", err)
	}

	// fetch account stats from broker
	balances, err := source.FetchEquity()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch equity: %w", err)
	}

	if balances.Equity < balance {
		return nil, fmt.Errorf("balance %.2f is greater than equity %.2f", balance, balances.Equity)
	}

	tradierTradesUrlTemplate, err := utils.GetEnv("TRADIER_TRADES_URL_TEMPLATE")
	if err != nil {
		return nil, fmt.Errorf("$TRADIER_TRADES_URL_TEMPLATE not set: %v", err)
	}

	tradesUrl := fmt.Sprintf(tradierTradesUrlTemplate, accountID)

	broker := NewTradierBroker(tradesUrl, apiKey)

	account := models.NewLiveAccount(balance, source, broker)

	return account, nil
}
