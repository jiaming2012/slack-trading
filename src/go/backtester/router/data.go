package router

import (
	"fmt"

	backtester_models "github.com/jiaming2012/slack-trading/src/go/backtester/models"
	"github.com/jiaming2012/slack-trading/src/go/data"
	"github.com/jiaming2012/slack-trading/src/go/models"
)

func loadData(dbService *data.DatabaseService, brokerMap map[backtester_models.CreateAccountRequestSource]backtester_models.IBroker, calendar *models.MarketCalendar) error {
	if err := dbService.LoadLiveAccounts(brokerMap); err != nil {
		return fmt.Errorf("loadData: failed to load live accounts: %w", err)
	}

	if err := dbService.LoadPlaygrounds(calendar); err != nil {
		return fmt.Errorf("loadData: failed to load playgrounds: %w", err)
	}

	return nil
}
