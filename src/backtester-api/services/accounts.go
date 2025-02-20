package services

import (
	"context"
	"fmt"

	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/utils"
)

type TradierBroker struct {
	ordersUrl      string
	quotesUrl      string
	nonTradesToken string
	tradesToken    string
}

func (b *TradierBroker) FetchQuotes(ctx context.Context, symbols []eventmodels.Instrument) ([]*models.TradierQuoteDTO, error) {
	if len(symbols) == 0 {
		return nil, fmt.Errorf("no symbols provided")
	}

	dto, err := FetchQuotes(ctx, b.quotesUrl, b.nonTradesToken, symbols)
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

func NewTradierBroker(ordersUrl, quotesUrl, nonTradesToken, tradesToken string) *TradierBroker {
	return &TradierBroker{
		ordersUrl:      ordersUrl,
		quotesUrl:      quotesUrl,
		nonTradesToken: nonTradesToken,
		tradesToken:    tradesToken,
	}
}

func CreateLiveAccount(balance float64, brokerName string, accountType models.LiveAccountType) (*models.LiveAccount, error) {
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
		AccountType:  &accountType,
		BalancesUrl:  balancesUrl,
		TradesApiKey: tradierTradesBearerToken,
	}

	if err := source.Validate(); err != nil {
		return nil, fmt.Errorf("invalid source: %w", err)
	}

	// balance check
	if balance > 0 {
		balances, err := source.FetchEquity()
		if err != nil {
			return nil, fmt.Errorf("failed to fetch equity: %w", err)
		}
	
		if balances.Equity < balance {
			return nil, fmt.Errorf("balance %.2f is greater than equity %.2f", balance, balances.Equity)
		}
	}

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

	account := models.NewLiveAccount(source, broker)

	return account, nil
}
