package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type IDatabaseService interface {
	GetPlaygrounds() []*Playground
	GetPlaygroundByClientId(clientId string) *Playground
	GetPlayground(playgroundID uuid.UUID) (*Playground, error)
	FetchReconcilePlayground(source CreateAccountRequestSource) (IReconcilePlayground, bool, error)
	FetchPlayground(playgroundId uuid.UUID) (*Playground, error)
	FindOrder(playgroundId uuid.UUID, id uint) (*Playground, *OrderRecord, error)
	FetchPendingOrders(accountType LiveAccountType) ([]*OrderRecord, error)
	DeletePlayground(playgroundID uuid.UUID) error
	CreateTransaction(transaction func(tx *gorm.DB) error) error
	PopulatePlayground(p *Playground) error
	PopulateLiveAccount(a *LiveAccount) error
	LoadLiveAccounts(brokerMap map[CreateAccountRequestSource]IBroker) error
	CreateRepos(repoRequests []eventmodels.CreateRepositoryRequest, from, to *eventmodels.PolygonDate, newCandlesQueue *eventmodels.FIFOQueue[*BacktesterCandle]) ([]*CandleRepository, *eventmodels.WebError)
	RemoveLiveRepository(repo *CandleRepository) error
	LoadPlaygrounds() error
	SavePlaygroundSession(playground *Playground) error
	SaveInMemoryPlayground(p *Playground) error
	SaveOrderRecord(order *OrderRecord, newBalance *float64, forceNew bool) error
	SaveLiveRepository(repo *CandleRepository) error
	UpdatePlaygroundSession(playgroundSession *Playground) error
}
