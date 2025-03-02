package models

import "github.com/jiaming2012/slack-trading/src/eventmodels"

type IBacktesterApiService interface {
	CreateRepos(repoRequests []eventmodels.CreateRepositoryRequest, from, to *eventmodels.PolygonDate, newCandlesQueue *eventmodels.FIFOQueue[*BacktesterCandle]) ([]*CandleRepository, *eventmodels.WebError)
	PopulatePlayground(p PlaygroundSession) (IPlayground, error)
	CreateLiveAccount(brokerName string, accountType LiveAccountType, reconcilePlayground *ReconcilePlayground) (*LiveAccount, error)
	RemoveLiveRepository(repo *CandleRepository) error
	SaveLiveRepository(repo *CandleRepository) error
}
