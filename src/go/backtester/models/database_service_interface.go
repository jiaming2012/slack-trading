package models

import (
	"time"

	"github.com/google/uuid"

	"github.com/jiaming2012/slack-trading/src/go/models"
)

type IDatabaseService interface {
	GetPlaygrounds() []*Playground
	GetPlaygroundByClientId(clientId string) *Playground
	GetPlayground(playgroundID uuid.UUID) (*Playground, error)
	GetLiveAccount(source CreateAccountRequestSource) (ILiveAccount, error)
	GetOrder(id uint) (*OrderRecord, error)
	GetOrdersByClientId(clientId string) ([]*OrderRecord, error)
	GetEquityPlots(playgroundId uuid.UUID) ([]LiveAccountPlot, error)
	FetchReconcilePlayground(source CreateAccountRequestSource) (IReconcilePlayground, bool, error)
	FetchReconcilePlaygroundByOrder(order *OrderRecord) (IReconcilePlayground, bool, error)
	FetchPlayground(playgroundId uuid.UUID) (*Playground, error)
	FetchNewOrders() (newOrders []*OrderRecord, err error)
	FetchExternalIdMap(orders []*OrderRecord) (map[uint]*OrderRecord, error)
	FindOrder(playgroundId uuid.UUID, id uint) (*Playground, *OrderRecord, error)
	RejectOrder(order *OrderRecord, reason string) error
	CancelOrder(order *OrderRecord) error
	FetchPendingOrders(accountTypes []AccountRole, seekFromPlayground bool) ([]*OrderRecord, error)
	DeletePlayground(playgroundID uuid.UUID) error
	CreatePlayground(playground *Playground, req *PopulatePlaygroundRequest) error
	PopulatePlayground(p *Playground, calendar *models.MarketCalendar) error
	PopulateLiveAccount(a *LiveAccount) error
	LoadLiveAccounts(brokerMap map[CreateAccountRequestSource]IBroker) error
	CreateRepos(repoRequests []models.CreateRepositoryRequest, from, to *models.PolygonDate, newCandlesQueue *models.FIFOQueue[*BacktesterCandle]) ([]*CandleRepository, *models.WebError)
	RemoveLiveRepository(repo *CandleRepository) error
	LoadPlaygrounds(calendar *models.MarketCalendar) error
	SavePlaygroundSession(playground *Playground) error
	SavePlaygroundInMemory(p *Playground) error
	SaveOrderRecord(order *OrderRecord, newBalance *float64, forceNew bool) error
	SaveOrderRecordIntents(intents []OrderSaveIntent) error
	SaveCanceledOrderWithReconciles(order *OrderRecord) error
	SaveRejectedOrderWithReconciles(order *OrderRecord) error
	SaveOrderRecords(order []*OrderRecord, forceNew bool) error
	SaveLiveRepository(repo *CandleRepository) error
	UpdatePlaygroundSession(playgroundSession *Playground) error
	FetchTradesFromReconciliationOrders(reconcileId uint, seekFromPlayground bool) ([]*TradeRecord, error)
	FetchReconciliationOrders(reconcileId uint, seekFromPlayground bool) ([]*OrderRecord, error)
	SaveEquityPlotRecord(playgroundId uuid.UUID, timestamp time.Time, equity float64) error
	PlaceOrders(playgroundID uuid.UUID, requests []*CreateOrderRequest) ([]*OrderRecord, error)

	// Deferred option auto-closes (wire-companion-stops): deferrals originate
	// from drain-once assignment/expiration events, so they are persisted on
	// deferral, deleted on successful placement, and reloaded at playground
	// load — a halt followed by a restart must not silently drop a close.
	SaveDeferredAutoClose(playgroundID uuid.UUID, d *DeferredAutoClose) error
	DeleteDeferredAutoClose(recordID uint) error
	LoadDeferredAutoCloses(playgroundID uuid.UUID) ([]*DeferredAutoClose, error)
}
