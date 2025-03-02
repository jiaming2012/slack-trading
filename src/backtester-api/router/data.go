package router

import (
	"fmt"

	"github.com/jiaming2012/slack-trading/src/backtester-api/services"
	"github.com/jiaming2012/slack-trading/src/data"
)

func loadData(apiService *services.BacktesterApiService, dbService *data.DatabaseService) error {
	if err := dbService.LoadLiveAccounts(apiService); err != nil {
		return fmt.Errorf("loadData: failed to load live accounts: %w", err)
	}

	if err := dbService.LoadPlaygrounds(apiService); err != nil {
		return fmt.Errorf("loadData: failed to load playgrounds: %w", err)
	}

	return nil
}
