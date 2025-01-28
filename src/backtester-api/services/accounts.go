package services

import (
	"context"
	"fmt"
	"os"

	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/utils"
)

type TradierBroker struct {
	ordersUrl   string
	quotesUrl   string
	token       string
	tradesToken string
}

func (b *TradierBroker) FetchQuotes(ctx context.Context, symbols []eventmodels.Instrument) ([]*models.TradierQuoteDTO, error) {
	if len(symbols) == 0 {
		return nil, fmt.Errorf("no symbols provided")
	}
	
	dto, err := FetchQuotes(ctx, b.quotesUrl, b.token, symbols)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch quotes: %w", err)
	}

	return dto, nil
}

func (b *TradierBroker) FetchOrders(ctx context.Context) ([]*eventmodels.TradierOrder, error) {
	dto, err := FetchOrders(ctx, b.ordersUrl, b.tradesToken)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch orders: %w", err)
	}

	orders := make([]*eventmodels.TradierOrder, 0, len(dto))
	for _, orderDTO := range dto {
		order, err := orderDTO.ToTradierOrder()
		if err != nil {
			return nil, fmt.Errorf("failed to convert order dto to order: %w", err)
		}

		orders = append(orders, order)
	}

	return orders, nil
}

func (b *TradierBroker) PlaceOrder(ctx context.Context, req *models.PlaceEquityTradeRequest) (map[string]interface{}, error) {

	resp, err := PlaceOrder(ctx, b.ordersUrl, b.tradesToken, req)
	if err != nil {
		return nil, fmt.Errorf("failed to place order: %w", err)
	}

	return resp, nil
}

func NewTradierBroker(ordersUrl, quotesUrl, token, tradesToken string) *TradierBroker {
	return &TradierBroker{
		ordersUrl:   ordersUrl,
		quotesUrl:   quotesUrl,
		token:       token,
		tradesToken: tradesToken,
	}
}

func CreateLiveAccount(balance float64, accountID, brokerName, apiKeyName string) (*models.LiveAccount, error) {
	if balance < 0 {
		return nil, fmt.Errorf("balance cannot be negative")
	}

	apiKey := os.Getenv(apiKeyName)
	if apiKey == "" {
		return nil, fmt.Errorf("cannot find apiKey with apiKeyName: %s", apiKeyName)
	}

	tradierBalancesUrlTemplate, err := utils.GetEnv("TRADIER_BALANCES_URL_TEMPLATE")
	if err != nil {
		return nil, fmt.Errorf("$TRADIER_BALANCES_URL_TEMPLATE not set: %v", err)
	}

	url := fmt.Sprintf(tradierBalancesUrlTemplate, accountID)

	source := LiveAccountSource{
		Broker:     brokerName,
		AccountID:  accountID,
		ApiKey:     apiKey,
		ApiKeyName: apiKeyName,
		Url:        url,
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

	stockQuotesURL, err := utils.GetEnv("STOCK_QUOTES_URL")
	if err != nil {
		return nil, fmt.Errorf("$STOCK_QUOTES_URL not set: %v", err)
	}

	tradierBearerToken, err := utils.GetEnv("TRADIER_BEARER_TOKEN")
	if err != nil {
		return nil, fmt.Errorf("$TRADIER_TRADES_BEARER_TOKEN not set: %v", err)
	}

	broker := NewTradierBroker(tradesUrl, stockQuotesURL, tradierBearerToken, apiKey)

	account := models.NewLiveAccount(balance, source, broker)

	return account, nil
}
