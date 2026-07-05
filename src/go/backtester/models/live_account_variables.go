package models

import (
	"fmt"

	"github.com/jiaming2012/slack-trading/src/go/utils"
)

type LiveAccountVariables struct {
	AccountType AccountRole
}

func (v LiveAccountVariables) GetTradierBalancesUrlTemplate() (tradierBalancesUrlTemplate string, err error) {
	switch v.AccountType {
	case AccountRolePaper:
		tradierBalancesUrlTemplate, err = utils.GetEnv("TRADIER_SANDBOX_BALANCES_URL_TEMPLATE")
	case AccountRoleMargin:
		tradierBalancesUrlTemplate, err = utils.GetEnv("TRADIER_LIVE_BALANCES_URL_TEMPLATE")
	default:
		tradierBalancesUrlTemplate = ""
		err = fmt.Errorf("LiveAccountVariables.GetTradierBalancesUrlTemplate: unsupported account type: %s", v.AccountType)
	}

	return
}

func (v LiveAccountVariables) GetTradierTradesOrderURL() (tradierTradesOrderURL string, err error) {
	tradierTradesOrderURL = ""

	tradesAccountID, e := v.GetTradierTradesAccountID()
	if e != nil {
		err = fmt.Errorf("LiveAccountVariables.GetTradierTradesOrderURL() failed: %w", e)
		return
	}

	tradierTradesUrlTemplate, e := v.GetTradierTradesUrlTemplate()
	if e != nil {
		err = fmt.Errorf("LiveAccountVariables.GetTradierTradesOrderURL() failed: %w", e)
		return
	}

	tradierTradesOrderURL = fmt.Sprintf(tradierTradesUrlTemplate, tradesAccountID)
	return
}

func (v LiveAccountVariables) GetPositionsUrlTemplate() (tradierPositionsOrderURL string, err error) {
	switch v.AccountType {
	case AccountRolePaper:
		tradierPositionsOrderURL, err = utils.GetEnv("TRADIER_SANDBOX_POSITIONS_URL_TEMPLATE")
	case AccountRoleMargin:
		tradierPositionsOrderURL, err = utils.GetEnv("TRADIER_LIVE_POSITIONS_URL_TEMPLATE")
	default:
		tradierPositionsOrderURL = ""
		err = fmt.Errorf("LiveAccountVariables.GetPositionsUrlTemplate: unsupported account type: %s", v.AccountType)
	}

	return
}

func (v LiveAccountVariables) GetTradierTradesUrlTemplate() (tradierTradesUrlTemplate string, err error) {
	switch v.AccountType {
	case AccountRolePaper:
		tradierTradesUrlTemplate, err = utils.GetEnv("TRADIER_SANDBOX_TRADES_URL_TEMPLATE")
	case AccountRoleMargin:
		tradierTradesUrlTemplate, err = utils.GetEnv("TRADIER_LIVE_TRADES_URL_TEMPLATE")
	default:
		tradierTradesUrlTemplate = ""
		err = fmt.Errorf("LiveAccountVariables.GetTradierTradesUrlTemplate: unsupported account type: %s", v.AccountType)
	}

	return
}

func (v LiveAccountVariables) GetTradierNonTradesBearerToken() (tradierNonTradesBearerToken string, err error) {
	switch v.AccountType {
	case AccountRolePaper:
		tradierNonTradesBearerToken, err = utils.GetEnv("TRADIER_SANDBOX_NON_TRADES_BEARER_TOKEN")
	case AccountRoleMargin:
		tradierNonTradesBearerToken, err = utils.GetEnv("TRADIER_LIVE_NON_TRADES_BEARER_TOKEN")
	default:
		tradierNonTradesBearerToken = ""
		err = fmt.Errorf("LiveAccountVariables.GetTradierNonTradesBearerToken: unsupported account type: %s", v.AccountType)
	}

	return
}

func (v LiveAccountVariables) GetTradierPositionsUrlTemplate() (tradierPositionsUrlTemplate string, err error) {
	switch v.AccountType {
	case AccountRolePaper:
		tradierPositionsUrlTemplate, err = utils.GetEnv("TRADIER_SANDBOX_POSITIONS_URL_TEMPLATE")
	case AccountRoleMargin:
		tradierPositionsUrlTemplate, err = utils.GetEnv("TRADIER_LIVE_POSITIONS_URL_TEMPLATE")
	default:
		tradierPositionsUrlTemplate = ""
		err = fmt.Errorf("LiveAccountVariables.GetTradierPositionsUrlTemplate: unsupported account type: %s", v.AccountType)
	}

	return
}

func (v LiveAccountVariables) GetTradierTradesAccountID() (accountID string, err error) {
	switch v.AccountType {
	case AccountRolePaper:
		accountID, err = utils.GetEnv("TRADIER_SANDBOX_TRADES_ACCOUNT_ID")
	case AccountRoleMargin:
		accountID, err = utils.GetEnv("TRADIER_LIVE_TRADES_ACCOUNT_ID")
	case AccountRoleMock:
		accountID = "mock_default"
		err = nil
	default:
		accountID = ""
		err = fmt.Errorf("LiveAccountVariables.GetTradierTradesAccountID: unsupported account type: %s", v.AccountType)
	}

	return
}

func (v LiveAccountVariables) GetTradierTradesBearerToken() (tradierTradesBearerToken string, err error) {
	switch v.AccountType {
	case AccountRolePaper:
		tradierTradesBearerToken, err = utils.GetEnv("TRADIER_SANDBOX_TRADES_BEARER_TOKEN")
	case AccountRoleMargin:
		tradierTradesBearerToken, err = utils.GetEnv("TRADIER_LIVE_TRADES_BEARER_TOKEN")
	default:
		tradierTradesBearerToken = ""
		err = fmt.Errorf("LiveAccountVariables.GetTradierTradesBearerToken: unsupported account type: %s", v.AccountType)
	}

	return
}

func (v LiveAccountVariables) GetTradierAccountID() (accountID string, err error) {
	switch v.AccountType {
	case AccountRolePaper:
		accountID, err = utils.GetEnv("TRADIER_SANDBOX_ACCOUNT_ID")
	case AccountRoleMargin:
		accountID, err = utils.GetEnv("TRADIER_LIVE_ACCOUNT_ID")
	default:
		accountID = ""
		err = fmt.Errorf("LiveAccountVariables.GetAccountID: unsupported account type: %s", v.AccountType)
	}

	return
}

func NewLiveAccountVariables(accountType AccountRole) LiveAccountVariables {
	return LiveAccountVariables{
		AccountType: accountType,
	}
}
