package models

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/jiaming2012/slack-trading/src/go/models"
)

type MockDatabase struct {
	orderRecords         map[uuid.UUID][]*OrderRecord
	playgrounds          map[uuid.UUID]*Playground
	reconcilePlaygrounds map[CreateAccountRequestSource]IReconcilePlayground
	liveAccounts         map[CreateAccountRequestSource]ILiveAccount
	orderNounce          uint
	tradeNounce          uint
}

func (m *MockDatabase) SaveEquityPlotRecord(playgroundId uuid.UUID, timestamp time.Time, equity float64) error {
	return nil
}

func (m *MockDatabase) PlaceOrder(playgroundID uuid.UUID, requests *CreateOrderRequest) (*OrderRecord, error) {
	orders, err := m.PlaceOrders(playgroundID, []*CreateOrderRequest{requests})
	if err != nil {
		return nil, fmt.Errorf("PlaceOrder: %w", err)
	}

	return orders[0], nil
}

func (m *MockDatabase) PlaceOrders(playgroundID uuid.UUID, requests []*CreateOrderRequest) ([]*OrderRecord, error) {
	playground, found := m.playgrounds[playgroundID]
	if !found {
		return nil, fmt.Errorf("MockDatabase: playground not found")
	}

	if len(requests) != 1 {
		return nil, fmt.Errorf("MockDatabase: not implemented yet - only one order request is supported in mock environment")
	}

	req := requests[0]

	id := uint(0)
	oRecord, err := NewOrderRecord(
		id,
		req.ExternalOrderID,
		req.ClientRequestID,
		playgroundID,
		req.Class,
		playground.Meta.Role,
		playground.GetCurrentTime(),
		req.Symbol,
		req.Side,
		req.Quantity,
		req.OrderType,
		req.Duration,
		req.RequestedPrice,
		req.Price,
		nil,
		OrderRecordStatusPending,
		req.Tag,
		req.CloseOrderId,
		req.IsSystemOrder,
		req.Attributes,
		nil, // previousBalance
	)

	if err != nil {
		return nil, fmt.Errorf("MockDatabase: failed to create new order record: %w", err)
	}

	changes, err := playground.PlaceOrder(oRecord)
	if err != nil {
		return nil, fmt.Errorf("MockDatabase: failed to place order: %w", err)
	}

	if err := CommitPlaceOrderChanges(m, changes); err != nil {
		return nil, fmt.Errorf("MockDatabase: failed to commit order change: %w", err)
	}

	return []*OrderRecord{oRecord}, nil
}

func (m *MockDatabase) GetEquityPlots(playgroundId uuid.UUID) ([]LiveAccountPlot, error) {
	return nil, nil
}

func (m *MockDatabase) FetchExternalIdMap(orders []*OrderRecord) (map[uint]*OrderRecord, error) {
	return nil, nil
}

func (m *MockDatabase) SaveOrderRecordIntents(intents []OrderSaveIntent) error {
	for _, intent := range intents {
		if err := m.SaveOrderRecord(intent.Order, nil, intent.ForceNew); err != nil {
			return err
		}
	}

	return nil
}

func (m *MockDatabase) SaveCanceledOrderWithReconciles(order *OrderRecord) error {
	if order == nil {
		return fmt.Errorf("MockDatabase: order is nil")
	}

	return nil
}

func (m *MockDatabase) SaveRejectedOrderWithReconciles(order *OrderRecord) error {
	if order == nil {
		return fmt.Errorf("MockDatabase: order is nil")
	}

	return nil
}

func (m *MockDatabase) CancelOrder(order *OrderRecord) error {
	if order == nil {
		return fmt.Errorf("MockDatabase: order is nil")
	}

	order.Cancel()
	return nil
}

func (m *MockDatabase) RejectOrder(order *OrderRecord, reason string) error {
	if order == nil {
		return fmt.Errorf("MockDatabase: order is nil")
	}

	order.Reject(fmt.Errorf("MockDatabase: %s", reason))

	return nil
}

func (m *MockDatabase) GetOrdersByClientId(clientId string) ([]*OrderRecord, error) {
	var result []*OrderRecord
	for _, orders := range m.orderRecords {
		for _, order := range orders {
			if *order.ClientRequestID == clientId {
				result = append(result, order)
			}
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("MockDatabase: order not found using clientId")
	}
	return result, nil
}

func (m *MockDatabase) GetOrder(id uint) (*OrderRecord, error) {
	for _, orders := range m.orderRecords {
		for _, order := range orders {
			if order.ID == id {
				return order, nil
			}
		}
	}

	return nil, fmt.Errorf("MockDatabase: order not found")
}

func (m *MockDatabase) CreatePlayground(playground *Playground, req *PopulatePlaygroundRequest) error {
	period := time.Minute
	source := models.CandleRepositorySource{
		Type: "test",
	}

	var symbol models.StockSymbol
	if len(req.Repositories) == 0 {
		symbol = models.NewStockSymbol("AAPL")
	} else if len(req.Repositories) == 1 {
		symbol = models.NewStockSymbol(req.Repositories[0].Symbol)
	} else {
		return fmt.Errorf("only one repository is supported in mock environment")
	}

	startDate, err := time.Parse("2006-01-02", req.Clock.StartDate)
	if err != nil {
		return fmt.Errorf("failed to parse start date: %v", err)
	}

	stopDate, err := time.Parse("2006-01-02", req.Clock.StopDate)
	if err != nil {
		return fmt.Errorf("failed to parse end date: %v", err)
	}

	clock := NewClock(startDate, stopDate, nil)

	feed := []*models.PolygonAggregateBarV2{
		{
			Timestamp: startDate,
			Open:      100.0,
			High:      101.0,
			Low:       99.0,
			Close:     100.5,
			Volume:    1000,
		},
	}

	repo, err := NewCandleRepository(symbol, period, feed, []string{}, nil, 0, source)
	if err != nil {
		return fmt.Errorf("failed to create mock candle repository: %v", err)
	}

	newTradesQueue := models.NewFIFOQueue[*TradeRecord]("newTradesQueue", 999)
	invalidOrdersQueue := models.NewFIFOQueue[*OrderRecord]("invalidOrdersQueue", 999)
	return PopulatePlayground(playground, req, clock, clock.CurrentTime, newTradesQueue, invalidOrdersQueue, req.Calendar, repo)
}

func (m *MockDatabase) FetchTradesFromReconciliationOrders(reconcileId uint, seekFromPlayground bool) ([]*TradeRecord, error) {
	var records []*TradeRecord

	for _, reconcilePlayground := range m.reconcilePlaygrounds {
		p := reconcilePlayground.GetPlayground()
		for _, order := range p.GetAllOrders() {
			for _, o := range order.Reconciles {
				if o.ID == reconcileId {
					records = append(records, order.Trades...)
					records = append(records, order.ReconcileTrades...)
				}
			}
		}
	}

	return records, nil
}

func (m *MockDatabase) FetchReconciliationOrders(reconcileId uint, seekFromPlayground bool) ([]*OrderRecord, error) {
	return nil, nil
}

func (m *MockDatabase) SetReconcilePlayground(source CreateAccountRequestSource, reconcilePlayground IReconcilePlayground) {
	m.reconcilePlaygrounds[source] = reconcilePlayground
}

func (m *MockDatabase) SaveOrderRecords(order []*OrderRecord, forceNew bool) error {
	for _, o := range order {
		if err := m.SaveOrderRecord(o, nil, forceNew); err != nil {
			return fmt.Errorf("failed to save order record: %w", err)
		}
	}

	return nil
}

func (m *MockDatabase) SaveOrderRecord(order *OrderRecord, newBalance *float64, forceNew bool) error {
	playgroundId := order.PlaygroundID

	if _, found := m.orderRecords[playgroundId]; !found {
		return fmt.Errorf("MockDatabase: playground not found in order records")
	}

	if order.ID == 0 {
		m.orderNounce++
		order.ID = m.orderNounce
	}

	for _, trade := range order.Trades {
		if trade.ID == 0 {
			m.tradeNounce++
			trade.ID = m.tradeNounce
		}
	}

	bFoundOrderRecord := false
	for idx, o := range m.orderRecords[playgroundId] {
		if order.ExternalOrderID != nil {
			if o.ExternalOrderID == order.ExternalOrderID {
				m.orderRecords[playgroundId][idx] = order
				bFoundOrderRecord = true
				break
			}
		} else {
			if o.ID == order.ID {
				m.orderRecords[playgroundId][idx] = order
				bFoundOrderRecord = true
				break
			}
		}
	}

	if !bFoundOrderRecord {
		m.orderRecords[playgroundId] = append(m.orderRecords[playgroundId], order)
	}

	return nil
}

func (m *MockDatabase) LoadPlaygrounds(calendar *models.MarketCalendar) error {
	return nil
}

func (m *MockDatabase) SavePlaygroundSession(playground *Playground) error {
	m.playgrounds[playground.GetId()] = playground
	m.orderRecords[playground.GetId()] = make([]*OrderRecord, 0)
	return nil
}

func (m *MockDatabase) GetLiveAccount(source CreateAccountRequestSource) (ILiveAccount, error) {
	return m.liveAccounts[source], nil
}

func (m *MockDatabase) SaveLiveAccount(source *CreateAccountRequestSource, liveAccount ILiveAccount) error {
	m.liveAccounts[*source] = liveAccount
	return nil
}

func (m *MockDatabase) UpdatePlaygroundSession(playgroundSession *Playground) error {
	return nil
}

func (m *MockDatabase) FetchLiveAccount(source *CreateAccountRequestSource) (ILiveAccount, bool, error) {
	return nil, false, nil
}

func (m *MockDatabase) FetchPlayground(playgroundId uuid.UUID) (*Playground, error) {
	playground, found := m.playgrounds[playgroundId]
	if !found {
		return nil, fmt.Errorf("MockDatabase: playground not found")
	}

	return playground, nil
}

func (m *MockDatabase) GetPlaygrounds() []*Playground {
	return nil
}

func (m *MockDatabase) GetPlaygroundByClientId(clientId string) *Playground {
	return nil
}

func (m *MockDatabase) GetPlayground(playgroundID uuid.UUID) (*Playground, error) {
	playground, found := m.playgrounds[playgroundID]
	if !found {
		return nil, fmt.Errorf("MockDatabase: playground not found")
	}

	return playground, nil
}

func (m *MockDatabase) DeletePlayground(playgroundID uuid.UUID) error {
	return nil
}

func (m *MockDatabase) SavePlaygroundInMemory(p *Playground) error {
	return nil
}

// FindOrder mirrors DatabaseService.FindOrder: it resolves the order by its
// EXTERNAL (broker) order id over the playground's in-memory orders — the id
// the live order-update pipeline carries in Tradier status events.
func (m *MockDatabase) FindOrder(playgroundId uuid.UUID, id uint) (*Playground, *OrderRecord, error) {
	playground, found := m.playgrounds[playgroundId]
	if !found {
		return nil, nil, fmt.Errorf("MockDatabase: playground not found")
	}

	for _, order := range playground.GetAllOrders() {
		if order.ExternalOrderID != nil && *order.ExternalOrderID == id {
			return playground, order, nil
		}
	}

	return nil, nil, fmt.Errorf("MockDatabase: order not found")
}

func (m *MockDatabase) FetchReconcilePlayground(source CreateAccountRequestSource) (IReconcilePlayground, bool, error) {
	p, found := m.reconcilePlaygrounds[source]
	return p, found, nil
}

func (m *MockDatabase) FetchReconcilePlaygroundByOrder(order *OrderRecord) (IReconcilePlayground, bool, error) {
	for _, reconcilePlayground := range m.reconcilePlaygrounds {
		if reconcilePlayground.GetId() == order.PlaygroundID {
			return reconcilePlayground, true, nil
		}
	}

	return nil, false, nil
}

func (m *MockDatabase) FetchNewOrders() (newOrders []*OrderRecord, err error) {
	for _, orders := range m.orderRecords {
		for _, order := range orders {
			if order.Status == OrderRecordStatusNew {
				newOrders = append(newOrders, order)
			}
		}
	}

	if len(newOrders) > 0 {
		return newOrders, nil
	}

	return nil, nil
}

func (m *MockDatabase) FetchPendingOrders(liveAccountTypes []AccountRole, seekFromPlayground bool) ([]*OrderRecord, error) {
	var orders []*OrderRecord

	for pId := range m.playgrounds {
		orderRecords := m.orderRecords[pId]
		for _, order := range orderRecords {
			if order.Status == OrderRecordStatusPending {
				for _, t := range liveAccountTypes {
					if order.AccountRole == t {
						orders = append(orders, order)
					}
				}
			}
		}
	}

	return orders, nil
}

func (m *MockDatabase) CreateRepos(repoRequests []models.CreateRepositoryRequest, from, to *models.PolygonDate, newCandlesQueue *models.FIFOQueue[*BacktesterCandle]) ([]*CandleRepository, *models.WebError) {
	return nil, nil
}

func (m *MockDatabase) RemoveLiveRepository(repo *CandleRepository) error {
	return nil
}

func (m *MockDatabase) SaveLiveRepository(repo *CandleRepository) error {
	return nil
}

func (m *MockDatabase) PopulatePlayground(p *Playground, calendar *models.MarketCalendar) error {
	return nil
}

func (m *MockDatabase) PopulateLiveAccount(l *LiveAccount) error {
	return nil
}

func (m *MockDatabase) LoadLiveAccounts(brokerMap map[CreateAccountRequestSource]IBroker) error {
	return nil
}

func NewMockDatabase() *MockDatabase {
	return &MockDatabase{
		orderRecords:         make(map[uuid.UUID][]*OrderRecord),
		playgrounds:          make(map[uuid.UUID]*Playground),
		reconcilePlaygrounds: make(map[CreateAccountRequestSource]IReconcilePlayground),
	}
}
