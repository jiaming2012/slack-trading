package dbutils

import (
	"fmt"
	"time"

	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func CreateReconcilePlayground(s models.IDatabaseService, source *models.CreateAccountRequestSource, createdAt time.Time) (*models.ReconcilePlayground, error) {
	if source == nil {
		return nil, fmt.Errorf("source is nil")
	}

	liveAccount, err := s.GetLiveAccount(*source)
	if err != nil {
		return nil, fmt.Errorf("failed to get broker: %v", err)
	}

	createPlaygroundReq := &models.PopulatePlaygroundRequest{
		Env: models.PlaygroundEnvironmentReconcile,
		Account: models.CreateAccountRequest{
			Source: source,
		},
		Repositories: nil,
		CreatedAt:    createdAt,
		LiveAccount:  liveAccount,
		SaveToDB:     true,
	}

	playground := &models.Playground{}
	if err := s.CreatePlayground(playground, createPlaygroundReq, nil); err != nil {
		return nil, fmt.Errorf("failed to create playground: %v", err)
	}

	reconcilePlayground, err := models.NewReconcilePlayground(playground, liveAccount)
	if err != nil {
		return nil, eventmodels.NewWebError(500, "failed to create new reconcile playground", err)
	}

	if err := s.UpdatePlaygroundSession(playground); err != nil {
		return nil, fmt.Errorf("failed to update playground session: %v", err)
	}

	return reconcilePlayground, nil
}
