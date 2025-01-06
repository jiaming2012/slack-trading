package services

import (
	"fmt"
	"os"

	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
	"github.com/jiaming2012/slack-trading/src/utils"
)

// var (
// 	playgroundIdToAccountsMap = map[uuid.UUID]*models.LiveAccount{}
// )

// func SavePlaygroundAccount(playground models.IPlayground, account *models.LiveAccount) {
// 	playgroundIdToAccountsMap[playground.GetId()] = account
// }

func CreateLiveAccount(balance float64, accountID, broker, apiKeyName string) (*models.LiveAccount, error) {
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
		Broker:    broker,
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

	account := models.NewLiveAccount(balance, source)

	return account, nil
}
