package models

import (
	"fmt"

	"github.com/jiaming2012/slack-trading/src/utils"
)

type LiveAccountVariables struct {
	AccountType LiveAccountType
}

func (v LiveAccountVariables) GetTradierBalancesUrlTemplate() (tradierBalancesUrlTemplate string, err error) {
	switch v.AccountType {
	case LiveAccountTypePaper:
		tradierBalancesUrlTemplate, err = utils.GetEnv("TRADIER_SANDBOX_BALANCES_URL_TEMPLATE")
	case LiveAccountTypeMargin:
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
	case LiveAccountTypePaper:
		tradierPositionsOrderURL, err = utils.GetEnv("TRADIER_SANDBOX_POSITIONS_URL_TEMPLATE")
	case LiveAccountTypeMargin:
		tradierPositionsOrderURL, err = utils.GetEnv("TRADIER_LIVE_POSITIONS_URL_TEMPLATE")
	default:
		tradierPositionsOrderURL = ""
		err = fmt.Errorf("LiveAccountVariables.GetPositionsUrlTemplate: unsupported account type: %s", v.AccountType)
	}

	return
}

func (v LiveAccountVariables) GetTradierTradesUrlTemplate() (tradierTradesUrlTemplate string, err error) {
	switch v.AccountType {
	case LiveAccountTypePaper:
		tradierTradesUrlTemplate, err = utils.GetEnv("TRADIER_SANDBOX_TRADES_URL_TEMPLATE")
	case LiveAccountTypeMargin:
		tradierTradesUrlTemplate, err = utils.GetEnv("TRADIER_LIVE_TRADES_URL_TEMPLATE")
	default:
		tradierTradesUrlTemplate = ""
		err = fmt.Errorf("LiveAccountVariables.GetTradierTradesUrlTemplate: unsupported account type: %s", v.AccountType)
	}

	return
}

func (v LiveAccountVariables) GetTradierNonTradesBearerToken() (tradierNonTradesBearerToken string, err error) {
	switch v.AccountType {
	case LiveAccountTypePaper:
		tradierNonTradesBearerToken, err = utils.GetEnv("TRADIER_SANDBOX_NON_TRADES_BEARER_TOKEN")
	case LiveAccountTypeMargin:
		tradierNonTradesBearerToken, err = utils.GetEnv("TRADIER_LIVE_NON_TRADES_BEARER_TOKEN")
	default:
		tradierNonTradesBearerToken = ""
		err = fmt.Errorf("LiveAccountVariables.GetTradierNonTradesBearerToken: unsupported account type: %s", v.AccountType)
	}

	return
}

func (v LiveAccountVariables) GetTradierPositionsUrlTemplate() (tradierPositionsUrlTemplate string, err error) {
	switch v.AccountType {
	case LiveAccountTypePaper:
		tradierPositionsUrlTemplate, err = utils.GetEnv("TRADIER_SANDBOX_POSITIONS_URL_TEMPLATE")
	case LiveAccountTypeMargin:
		tradierPositionsUrlTemplate, err = utils.GetEnv("TRADIER_LIVE_POSITIONS_URL_TEMPLATE")
	default:
		tradierPositionsUrlTemplate = ""
		err = fmt.Errorf("LiveAccountVariables.GetTradierPositionsUrlTemplate: unsupported account type: %s", v.AccountType)
	}

	return
}

func (v LiveAccountVariables) GetTradierTradesAccountID() (accountID string, err error) {
	switch v.AccountType {
	case LiveAccountTypePaper:
		accountID, err = utils.GetEnv("TRADIER_SANDBOX_TRADES_ACCOUNT_ID")
	case LiveAccountTypeMargin:
		accountID, err = utils.GetEnv("TRADIER_LIVE_TRADES_ACCOUNT_ID")
	default:
		accountID = ""
		err = fmt.Errorf("LiveAccountVariables.GetTradierTradesAccountID: unsupported account type: %s", v.AccountType)
	}

	return
}

func (v LiveAccountVariables) GetTradierTradesBearerToken() (tradierTradesBearerToken string, err error) {
	switch v.AccountType {
	case LiveAccountTypePaper:
		tradierTradesBearerToken, err = utils.GetEnv("TRADIER_SANDBOX_TRADES_BEARER_TOKEN")
	case LiveAccountTypeMargin:
		tradierTradesBearerToken, err = utils.GetEnv("TRADIER_LIVE_TRADES_BEARER_TOKEN")
	default:
		tradierTradesBearerToken = ""
		err = fmt.Errorf("LiveAccountVariables.GetTradierTradesBearerToken: unsupported account type: %s", v.AccountType)
	}

	return
}

func (v LiveAccountVariables) GetTradierAccountID() (accountID string, err error) {
	switch v.AccountType {
	case LiveAccountTypePaper:
		accountID, err = utils.GetEnv("TRADIER_SANDBOX_ACCOUNT_ID")
	case LiveAccountTypeMargin:
		accountID, err = utils.GetEnv("TRADIER_LIVE_ACCOUNT_ID")
	default:
		accountID = ""
		err = fmt.Errorf("LiveAccountVariables.GetAccountID: unsupported account type: %s", v.AccountType)
	}

	return
}

func NewLiveAccountVariables(accountType LiveAccountType) LiveAccountVariables {
	return LiveAccountVariables{
		AccountType: accountType,
	}
}
