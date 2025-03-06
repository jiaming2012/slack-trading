package router

import (
	"fmt"

	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
	"github.com/jiaming2012/slack-trading/src/data"
)

func loadData(dbService *data.DatabaseService, brokerMap map[models.CreateAccountRequestSource]models.IBroker) error {
	if err := dbService.LoadLiveAccounts(brokerMap); err != nil {
		return fmt.Errorf("loadData: failed to load live accounts: %w", err)
	}

	if err := dbService.LoadPlaygrounds(); err != nil {
		return fmt.Errorf("loadData: failed to load playgrounds: %w", err)
	}

	return nil
}
