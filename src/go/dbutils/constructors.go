package dbutils

import (
	"fmt"
	"time"

	backtester_models "github.com/jiaming2012/slack-trading/src/go/backtester/models"
	"github.com/jiaming2012/slack-trading/src/go/models"
)

func CreateReconcilePlayground(s backtester_models.IDatabaseService, source *backtester_models.CreateAccountRequestSource, createdAt time.Time) (*backtester_models.ReconcilePlayground, error) {
	if source == nil {
		return nil, fmt.Errorf("source is nil")
	}

	liveAccount, err := s.GetLiveAccount(*source)
	if err != nil {
		return nil, fmt.Errorf("failed to get broker: %v", err)
	}

	createPlaygroundReq := &backtester_models.PopulatePlaygroundRequest{
		Env: backtester_models.PlaygroundEnvironmentReconcile,
		Account: backtester_models.CreateAccountRequest{
			Source: source,
		},
		Repositories: nil,
		CreatedAt:    createdAt,
		LiveAccount:  liveAccount,
		SaveToDB:     true,
	}

	playground := &backtester_models.Playground{}
	if err := s.CreatePlayground(playground, createPlaygroundReq); err != nil {
		return nil, fmt.Errorf("failed to create reconcile playground: %v", err)
	}

	reconcilePlayground, err := backtester_models.NewReconcilePlayground(playground, liveAccount)
	if err != nil {
		return nil, models.NewWebError(500, "failed to create new reconcile playground", err)
	}

	if err := s.UpdatePlaygroundSession(playground); err != nil {
		return nil, fmt.Errorf("failed to update playground session: %v", err)
	}

	return reconcilePlayground, nil
}
