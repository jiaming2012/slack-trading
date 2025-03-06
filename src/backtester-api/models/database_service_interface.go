package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type IDatabaseService interface {
	SaveOrderRecord(order *OrderRecord, newBalance *float64, forceNew bool) error
	LoadPlaygrounds() error
	SavePlaygroundSession(playground *Playground) error
	// SaveLiveAccount(source *CreateAccountRequestSource, liveAccount ILiveAccount) error
	UpdatePlaygroundSession(playgroundSession *Playground) error
	FetchReconcilePlayground(source CreateAccountRequestSource) (IReconcilePlayground, bool, error)
	// FetchLiveAccount(source *CreateAccountRequestSource) (ILiveAccount, bool, error)
	FetchPlayground(playgroundId uuid.UUID) (*Playground, error)
	GetPlaygrounds() []*Playground
	GetPlaygroundByClientId(clientId string) *Playground
	GetPlayground(playgroundID uuid.UUID) (*Playground, error)
	DeletePlayground(playgroundID uuid.UUID) error
	SaveInMemoryPlayground(p *Playground) error
	FindOrder(playgroundId uuid.UUID, id uint) (*Playground, *OrderRecord, error)
	FetchPendingOrders(accountType LiveAccountType) ([]*OrderRecord, error)
	CreateTransaction(transaction func(tx *gorm.DB) error) error
	PopulatePlayground(p *Playground) error
	CreateRepos(repoRequests []eventmodels.CreateRepositoryRequest, from, to *eventmodels.PolygonDate, newCandlesQueue *eventmodels.FIFOQueue[*BacktesterCandle]) ([]*CandleRepository, *eventmodels.WebError)
	CreateLiveAccount(broker IBroker, accountType LiveAccountType) (*LiveAccount, error)
	RemoveLiveRepository(repo *CandleRepository) error
	SaveLiveRepository(repo *CandleRepository) error
}
