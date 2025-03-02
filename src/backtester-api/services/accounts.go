package services

import (
	"fmt"

	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
	"github.com/jiaming2012/slack-trading/src/utils"
)

func (s *BacktesterApiService) CreateLiveAccount(brokerName string, accountType models.LiveAccountType, reconcilePlayground *models.ReconcilePlayground) (*models.LiveAccount, error) {
	if brokerName != "tradier" {
		return nil, fmt.Errorf("unsupported broker: %s", brokerName)
	}

	// if balance < 0 {
	// 	return nil, fmt.Errorf("balance cannot be negative")
	// }

	vars := models.NewLiveAccountVariables(accountType)

	tradierBalancesUrlTemplate, err := vars.GetTradierBalancesUrlTemplate()
	if err != nil {
		return nil, fmt.Errorf("failed to get tradier balances url template: %w", err)
	}

	accountID, err := vars.GetTradierTradesAccountID()
	if err != nil {
		return nil, fmt.Errorf("failed to get tradier account id: %w", err)
	}

	balancesUrl := fmt.Sprintf(tradierBalancesUrlTemplate, accountID)

	tradierTradesBearerToken, err := vars.GetTradierTradesBearerToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get tradier trades bearer token: %w", err)
	}

	source := LiveAccountSource{
		Broker:       brokerName,
		AccountID:    accountID,
		AccountType:  accountType,
		BalancesUrl:  balancesUrl,
		TradesApiKey: tradierTradesBearerToken,
	}

	if err := source.Validate(); err != nil {
		return nil, fmt.Errorf("invalid source: %w", err)
	}

	// balance check
	// if balance > 0 {
	// 	balances, err := source.FetchEquity()
	// 	if err != nil {
	// 		return nil, fmt.Errorf("failed to fetch equity: %w", err)
	// 	}

	// 	if balances.Equity < balance {
	// 		return nil, fmt.Errorf("balance %.2f is greater than equity %.2f", balance, balances.Equity)
	// 	}
	// }

	tradierTradesUrlTemplate, err := vars.GetTradierTradesUrlTemplate()
	if err != nil {
		return nil, fmt.Errorf("failed to get tradier trades url template: %w", err)
	}

	tradesUrl := fmt.Sprintf(tradierTradesUrlTemplate, accountID)

	stockQuotesURL, err := utils.GetEnv("TRADIER_STOCK_QUOTES_URL")
	if err != nil {
		return nil, fmt.Errorf("$TRADIER_STOCK_QUOTES_URL not set: %v", err)
	}

	tradierNonTradesBearerToken, err := vars.GetTradierNonTradesBearerToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get tradier non trades bearer token: %w", err)
	}

	broker := NewTradierBroker(tradesUrl, stockQuotesURL, tradierNonTradesBearerToken, tradierTradesBearerToken)

	account, err := models.NewLiveAccount(source, broker, reconcilePlayground)
	if err != nil {
		return nil, fmt.Errorf("failed to create live account: %w", err)
	}

	return account, nil
}
