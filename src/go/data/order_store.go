package data

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/jiaming2012/slack-trading/src/go/backtester/models"
	"github.com/jiaming2012/slack-trading/src/go/telemetry"
)

const FetchTradesFromReconciliationOrdersSql = `
SELECT
  t.*
FROM order_reconciles orr
JOIN order_records orec on orr.order_record_id = orec.id
JOIN trade_records t on orec.id = t.reconcile_order_id
WHERE orr.reconcile_id = $1 AND t.deleted_at IS NULL
`

const FetchReconciliationOrderSql = `
SELECT orec.*
FROM order_reconciles orr
JOIN order_records orec on orr.order_record_id = orec.id
WHERE orr.reconcile_id = $1 AND orec.deleted_at IS NULL
`

const FetchReconciliationOrdersSql = `
SELECT o1.reconcile_id, o2.* from order_reconciles o1
  JOIN order_records o2 on o1.order_record_id = o2.id
  WHERE reconcile_id in ?
`

const FetchMockMaxExternalIdSql = `
SELECT max(orec.external_id)
  FROM order_records orec
  JOIN order_reconciles or2 on or2.order_record_id = orec.id
  JOIN order_records orec2 on orec2.id = or2.reconcile_id
  WHERE orec2.account_type = 'mock'
`

const FetchMockOrderCountSql = `
SELECT count(id)
  FROM order_records orec
  WHERE account_type = 'mock'
`

type ReconcileOrderRecord struct {
	models.OrderRecord
	ReconcileId uint `gorm:"column:reconcile_id" copier:"must,nopanic"`
}

// orderStore owns the in-memory order and trade caches plus the order/trade
// GORM queries.
//
// Locking: the store performs NO locking of its own. Callers (the DatabaseService
// public methods) hold the service-wide mutex exactly as they did before the
// split — locking granularity was intentionally left unchanged. Do not add
// per-store locking without auditing every call site.
type orderStore struct {
	db          *gorm.DB
	ordersCache map[uint]*models.OrderRecord
	tradesCache map[uint]*models.TradeRecord
}

func newOrderStore(db *gorm.DB) *orderStore {
	return &orderStore{
		db:          db,
		ordersCache: make(map[uint]*models.OrderRecord),
		tradesCache: make(map[uint]*models.TradeRecord),
	}
}

func (st *orderStore) getOrdersByClientId(clientId string) ([]*models.OrderRecord, error) {
	var orders []*models.OrderRecord
	if err := st.db.Where("client_request_id = ?", clientId).Error; err != nil {
		return nil, fmt.Errorf("failed to get order: %w", err)
	}

	if len(orders) == 0 {
		return []*models.OrderRecord{}, nil
	}

	return orders, nil
}

func (st *orderStore) getOrder(id uint) (*models.OrderRecord, error) {
	var order *models.OrderRecord
	if err := st.db.Where("id = ?", id).First(&order).Error; err != nil {
		return nil, fmt.Errorf("failed to get order: %w", err)
	}

	if order == nil {
		return nil, fmt.Errorf("failed to find order with id: %d", id)
	}

	return order, nil
}

func (st *orderStore) deleteMockOrders() error {
	// Soft delete mock orders
	if err := st.db.Exec("UPDATE order_records SET deleted_at = NOW() WHERE account_type = 'mock'").Error; err != nil {
		return fmt.Errorf("failed to soft delete mock orders: %w", err)
	}

	return nil
}

func (st *orderStore) getMockOrderIdStartIndex() (uint, error) {
	var mockOrderCount uint
	if err := st.db.Raw(FetchMockOrderCountSql).Scan(&mockOrderCount).Error; err != nil {
		return 0, fmt.Errorf("failed to get mock order count: %w", err)
	}

	if mockOrderCount == 0 {
		return 1, nil
	}

	var mockMaxExternalId uint
	if err := st.db.Raw(FetchMockMaxExternalIdSql).Scan(&mockMaxExternalId).Error; err != nil {
		log.Errorf("failed to get mock order id start index: %v", err)
		log.Debug("setting mockMaxExternalId to 0")
		mockMaxExternalId = 0
	}

	return mockMaxExternalId + 1, nil
}

func (st *orderStore) fetchExternalIdMap(orders []*models.OrderRecord) (map[uint]*models.OrderRecord, error) {
	// var orderIds strings.Builder
	// for i := 0; i < len(orders)-1; i++ {
	// 	orderIds.WriteString(fmt.Sprintf("%d,", orders[i].ID))
	// }
	var orderIds []uint
	for _, o := range orders {
		orderIds = append(orderIds, o.ID)
	}

	// orderIds.WriteString(fmt.Sprintf("%d", orders[len(orders)-1].ID))

	var reconcileOrders []*ReconcileOrderRecord
	if err := st.db.Raw(FetchReconciliationOrdersSql, orderIds).Scan(&reconcileOrders).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch reconciliation orders: %w", err)
	}

	externalIdMap := make(map[uint]*models.OrderRecord)

	for _, o := range reconcileOrders {
		externalIdMap[o.ReconcileId] = &o.OrderRecord
	}

	return externalIdMap, nil
}

func (st *orderStore) fetchReconciliationOrders(reconcileId uint, seekFromPlayground bool) ([]*models.OrderRecord, error) {
	var orders []*models.OrderRecord
	if err := st.db.Raw(FetchReconciliationOrderSql, reconcileId).Scan(&orders).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch reconciliation orders: %w", err)
	}

	if seekFromPlayground {
		return st.seekOrdersFromPlayground(orders)
	}

	return orders, nil
}

func (st *orderStore) fetchTradesFromReconciliationOrders(reconcileId uint, seekFromPlayground bool) ([]*models.TradeRecord, error) {
	var trades []*models.TradeRecord
	if err := st.db.Raw(FetchTradesFromReconciliationOrdersSql, reconcileId).Scan(&trades).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch trades from reconciliation orders: %w", err)
	}

	if seekFromPlayground {
		return st.seekTradesFromPlayground(trades)
	}

	return trades, nil
}

func (st *orderStore) seekOrdersFromPlayground(orders []*models.OrderRecord) ([]*models.OrderRecord, error) {
	var out []*models.OrderRecord

	for _, o := range orders {
		o2, found := st.ordersCache[o.ID]
		if !found {
			log.Errorf("failed to find order in memory: %d, excluding from results ...", o.ID)
			continue
		}

		out = append(out, o2)
	}

	return out, nil
}

func (st *orderStore) seekTradesFromPlayground(trades []*models.TradeRecord) ([]*models.TradeRecord, error) {
	var out []*models.TradeRecord

	for _, t := range trades {
		t2, found := st.tradesCache[t.ID]
		if !found {
			log.Errorf("failed to find trade in memory: %d, excluding from results ...", t.ID)
			continue
		}

		out = append(out, t2)
	}

	return out, nil
}

func (st *orderStore) fetchNewOrders() (newOrders []*models.OrderRecord, err error) {
	var orders []*models.OrderRecord

	if err := st.db.Where("status = ?", string(models.OrderRecordStatusNew)).Find(&orders).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to fetch new order: %w", err)
	}

	if len(orders) > 0 {
		return orders, nil
	}

	return nil, nil
}

func (st *orderStore) fetchPendingOrders(liveAccountTypes []models.LiveAccountType) ([]*models.OrderRecord, error) {
	var orders []*models.OrderRecord

	if err := st.db.Joins("JOIN playground_sessions ON playground_sessions.id = order_records.playground_id").
		Where("playground_sessions.deleted_at IS NULL and order_records.status = ? and order_records.account_type in (?)", string(models.OrderRecordStatusPending), liveAccountTypes).Find(&orders).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch pending orders: %w", err)
	}

	for _, o := range orders {
		if o.ID == 612 {
			log.Infof("found order: %d", o.ID)
		}
	}

	return orders, nil
}

func (st *orderStore) waitForOrderRecord(orderID uint) error {
	timeout := time.After(10 * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for order %d", orderID)
		case <-ticker.C:
			var order models.OrderRecord
			result := st.db.First(&order, orderID)
			if result.Error == nil && result.RowsAffected > 0 {
				return nil // order found
			}
		}
	}
}

func (st *orderStore) cancelOrder(playground *models.Playground, order *models.OrderRecord) error {
	if err := st.db.Model(order).Update("status", models.OrderRecordStatusCanceled).Error; err != nil {
		return fmt.Errorf("CancelOrder: failed to update order status to cancelled: %w", err)
	}

	order.Status = models.OrderRecordStatusCanceled

	if err := playground.AddToOrderQueue(order); err != nil {
		return fmt.Errorf("CancelOrder: failed to add order to queue: %w", err)
	}

	return nil
}

func (st *orderStore) rejectOrder(playground *models.Playground, order *models.OrderRecord, reason string) error {
	err := st.db.Transaction(func(tx *gorm.DB) error {
		if err := st.db.Model(order).Update("status", models.OrderRecordStatusRejected).Error; err != nil {
			return fmt.Errorf("RejectOrder: failed to update order status to rejected: %w", err)
		}

		if err := st.db.Model(order).Update("reject_reason", reason).Error; err != nil {
			return fmt.Errorf("RejectOrder: failed to update order reject reason: %w", err)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("RejectOrder: failed to update order in transaction: %w", err)
	}

	order.Status = models.OrderRecordStatusRejected
	order.RejectReason = &reason

	if telemetry.ShouldEmitOrderTelemetry(string(playground.Meta.Environment)) {
		log.WithFields(log.Fields{
			"event":         "order_rejected",
			"playground_id": playground.GetId().String(),
			"order_id":      order.ID,
			"symbol":        order.Symbol,
			"reject_reason": reason,
			"environment":   string(playground.Meta.Environment),
			"account_type":  string(playground.Meta.LiveAccountType),
			"client_id":     telemetry.ClientIDOrEmpty(playground.GetClientId()),
		}).Warn("order rejected")

		if telemetry.OrdersRejected != nil {
			telemetry.OrdersRejected.Add(context.Background(), 1, telemetry.PlaygroundAttrs(string(playground.Meta.Environment), string(playground.Meta.LiveAccountType), telemetry.ClientIDOrEmpty(playground.GetClientId())))
		}
	}

	if err := playground.AddToOrderQueue(order); err != nil {
		return fmt.Errorf("RejectOrder: failed to add order to queue: %w", err)
	}

	return nil
}

func (st *orderStore) saveOrderRecords(orders []*models.OrderRecord, forceNew bool) error {
	return saveOrderRecordsTx(st.db, orders, forceNew)
}

func (st *orderStore) saveOrderRecordTx(tx *gorm.DB, order *models.OrderRecord, forceNew bool) error {
	if err := saveOrderRecordsTx(tx, []*models.OrderRecord{order}, forceNew); err != nil {
		return fmt.Errorf("saveOrderRecordTx: failed to save order record: %w", err)
	}

	return nil
}

func (st *orderStore) saveOrderRecord(order *models.OrderRecord, newBalance *float64, forceNew bool) error {
	err := st.db.Transaction(func(tx *gorm.DB) error {
		var e error
		if e = saveOrderRecordsTx(tx, []*models.OrderRecord{order}, forceNew); e != nil {
			return fmt.Errorf("saveOrderRecord: failed to save order records: %w", e)
		}

		if newBalance != nil {
			if e := saveBalance(tx, order.PlaygroundID, *newBalance); e != nil {
				return fmt.Errorf("saveOrderRecord: failed to save balance: %w", e)
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("saveOrderRecord: save order record transaction failed: %w", err)
	}

	log.Infof("SaveOrderRecord: order record: %d saved to db", order.ID)

	// save in cache
	st.ordersCache[order.ID] = order

	for _, t := range order.Trades {
		st.tradesCache[t.ID] = t
	}

	for _, t := range order.ReconcileTrades {
		st.tradesCache[t.ID] = t
	}

	return nil
}

// --- DatabaseService public surface: thin delegations to orderStore ---

func (s *DatabaseService) GetOrdersByClientId(clientId string) ([]*models.OrderRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.orderStore.getOrdersByClientId(clientId)
}

func (s *DatabaseService) GetOrder(id uint) (*models.OrderRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.orderStore.getOrder(id)
}

func (s *DatabaseService) DeleteMockOrders() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.orderStore.deleteMockOrders()
}

func (s *DatabaseService) GetMockOrderIdStartIndex() (uint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.orderStore.getMockOrderIdStartIndex()
}

func (s *DatabaseService) FetchExternalIdMap(orders []*models.OrderRecord) (map[uint]*models.OrderRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.orderStore.fetchExternalIdMap(orders)
}

func (s *DatabaseService) FetchReconciliationOrders(reconcileId uint, seekFromPlayground bool) ([]*models.OrderRecord, error) {
	return s.orderStore.fetchReconciliationOrders(reconcileId, seekFromPlayground)
}

func (s *DatabaseService) FetchTradesFromReconciliationOrders(reconcileId uint, seekFromPlayground bool) ([]*models.TradeRecord, error) {
	return s.orderStore.fetchTradesFromReconciliationOrders(reconcileId, seekFromPlayground)
}

func (s *DatabaseService) FetchNewOrders() (newOrders []*models.OrderRecord, err error) {
	return s.orderStore.fetchNewOrders()
}

func (s *DatabaseService) FetchPendingOrders(liveAccountTypes []models.LiveAccountType, seekFromPlayground bool) ([]*models.OrderRecord, error) {
	return s.orderStore.fetchPendingOrders(liveAccountTypes)
}

func (s *DatabaseService) CancelOrder(order *models.OrderRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	playground, err := s.playgroundStore.fetchPlayground(order.PlaygroundID)
	if err != nil {
		return fmt.Errorf("CancelOrder: failed to fetch playground: %w", err)
	}

	return s.orderStore.cancelOrder(playground, order)
}

func (s *DatabaseService) RejectOrder(order *models.OrderRecord, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	playground, err := s.playgroundStore.fetchPlayground(order.PlaygroundID)
	if err != nil {
		return fmt.Errorf("RejectOrder: failed to fetch playground: %w", err)
	}

	return s.orderStore.rejectOrder(playground, order, reason)
}

func (s *DatabaseService) SaveOrderRecords(orders []*models.OrderRecord, forceNew bool) error {
	return s.orderStore.saveOrderRecords(orders, forceNew)
}

// SaveOrderRecordTx accepts a caller-supplied *gorm.DB transaction — part of the
// IDatabaseService gorm.DB leak into the models package (see database_service_interface.go).
// Closing that leak is out of scope for this refactor.
func (s *DatabaseService) SaveOrderRecordTx(tx *gorm.DB, order *models.OrderRecord, forceNew bool) error {
	return s.orderStore.saveOrderRecordTx(tx, order, forceNew)
}

func (s *DatabaseService) SaveOrderRecord(order *models.OrderRecord, newBalance *float64, forceNew bool) error {
	return s.orderStore.saveOrderRecord(order, newBalance, forceNew)
}
