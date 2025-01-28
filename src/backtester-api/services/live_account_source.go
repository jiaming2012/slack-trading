package services

import (
	"fmt"

	// "github.com/jiaming2012/slack-trading/src/backtester-api/models"
	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/eventservices"
)

type LiveAccountSource struct {
	Broker       string                 `json:"broker"`
	AccountID    string                 `json:"account_id"`
	AccountType  models.LiveAccountType `json:"account_type"`
	BalancesUrl  string                 `json:"-"`
	TradesApiKey string                 `json:"-"`
}

func (s LiveAccountSource) GetAccountType() models.LiveAccountType {
	return s.AccountType
}

func (s LiveAccountSource) GetBroker() string {
	return s.Broker
}

func (s LiveAccountSource) GetAccountID() string {
	return s.AccountID
}

func (s LiveAccountSource) GetApiKey() string {
	return s.TradesApiKey
}

func (s LiveAccountSource) GetBrokerUrl() string {
	return s.BalancesUrl
}

func (s LiveAccountSource) Validate() error {
	if s.Broker != "tradier" {
		return fmt.Errorf("unsupported broker: %s", s.Broker)
	}

	return nil
}

func (s LiveAccountSource) FetchEquity() (*eventmodels.FetchAccountEquityResponse, error) {
	responseDTO, err := eventservices.FetchTradierBalances(s.BalancesUrl, s.TradesApiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch equity: %w", err)
	}

	switch responseDTO.Balances.AccountType {
	case "margin":
		break
	case "pdt": // Pattern Day Trading account
		break
	default:
		return nil, fmt.Errorf("unsupported account type: %s", responseDTO.Balances.AccountType)
	}

	return &eventmodels.FetchAccountEquityResponse{
		Equity:  responseDTO.Balances.TotalEquity,
		OpenPL:  responseDTO.Balances.OpenPL,
		ClosePL: responseDTO.Balances.ClosePL,
	}, nil
}
