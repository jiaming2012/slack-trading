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
	GetLiveAccount(source CreateAccountRequestSource) (ILiveAccount, error)
	FetchReconcilePlayground(source CreateAccountRequestSource) (IReconcilePlayground, bool, error)
	FetchReconcilePlaygroundByOrder(order *OrderRecord) (IReconcilePlayground, bool, error)
	FetchPlayground(playgroundId uuid.UUID) (*Playground, error)
	FetchNewOrder() (newOrder *OrderRecord, err error)
	FindOrder(playgroundId uuid.UUID, id uint) (*Playground, *OrderRecord, error)
	FetchPendingOrders(accountTypes []LiveAccountType, seekFromPlayground bool) ([]*OrderRecord, error)
	DeletePlayground(playgroundID uuid.UUID) error
	CreatePlayground(playground *Playground, req *PopulatePlaygroundRequest) error
	CreateTransaction(transaction func(tx *gorm.DB) error) error
	PopulatePlayground(p *Playground) error
	PopulateLiveAccount(a *LiveAccount) error
	LoadLiveAccounts(brokerMap map[CreateAccountRequestSource]IBroker) error
	CreateRepos(repoRequests []eventmodels.CreateRepositoryRequest, from, to *eventmodels.PolygonDate, newCandlesQueue *eventmodels.FIFOQueue[*BacktesterCandle]) ([]*CandleRepository, *eventmodels.WebError)
	RemoveLiveRepository(repo *CandleRepository) error
	LoadPlaygrounds() error
	SavePlaygroundSession(playground *Playground) error
	SavePlaygroundInMemory(p *Playground) error
	SaveOrderRecord(order *OrderRecord, newBalance *float64, forceNew bool) error
	SaveOrderRecords(order []*OrderRecord, forceNew bool) error
	SaveLiveRepository(repo *CandleRepository) error
	UpdatePlaygroundSession(playgroundSession *Playground) error
	FetchTradesFromReconciliationOrders(reconcileId uint, seekFromPlayground bool) ([]*TradeRecord, error)
	FetchReconciliationOrders(reconcileId uint, seekFromPlayground bool) ([]*OrderRecord, error)
}
