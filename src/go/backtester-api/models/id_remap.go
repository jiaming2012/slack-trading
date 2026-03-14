package models

import (
	"fmt"
	"sort"

	"gorm.io/gorm"
)

// remapState tracks old→new ID mappings during a playground save operation.
type remapState struct {
	orderIDMap map[uint]uint // old order ID → new order ID
	tradeIDMap map[uint]uint // old trade ID → new trade ID
}

// deferredCloseOrderID records an order that had a CloseOrderId pointing to another order.
type deferredCloseOrderID struct {
	order          *OrderRecord
	oldCloseOrderID uint
}

// deferredParentTradeID records a trade whose ParentTradeID pointed to another trade.
type deferredParentTradeID struct {
	trade          *TradeRecord
	oldParentTradeID uint
}

// RemapAndSavePlayground zeroes all in-memory nonce IDs on a simulator playground's
// orders and trades, saves them via GORM (which assigns fresh auto-increment IDs),
// then fixes up cross-references (CloseOrderId, ParentTradeID, M2M join tables).
//
// This allows simulator playgrounds that used BacktesterAccount nonces during the
// fast in-memory tick loop to be persisted to Postgres without ID collisions.
func RemapAndSavePlayground(tx *gorm.DB, playground *Playground) error {
	orders := playground.GetAllOrders()
	if len(orders) == 0 {
		// No orders — just save the playground record and equity plots
		if err := tx.Save(playground).Error; err != nil {
			return fmt.Errorf("RemapAndSavePlayground: failed to save playground: %w", err)
		}
		return saveEquityPlots(tx, playground)
	}

	state := &remapState{
		orderIDMap: make(map[uint]uint),
		tradeIDMap: make(map[uint]uint),
	}

	// Collect deferred fixups and M2M associations before zeroing IDs
	var deferredCloseOrders []deferredCloseOrderID
	var deferredParentTrades []deferredParentTradeID

	type m2mAssoc struct {
		order     *OrderRecord
		closes    []*OrderRecord
		closedBy  []*TradeRecord
		reconciles []*OrderRecord
	}
	var m2mFixups []m2mAssoc

	// Collect all unique trades across all orders
	allTrades := make(map[uint]*TradeRecord)

	for _, order := range orders {
		// Record M2M associations before stripping
		assoc := m2mAssoc{order: order}
		if len(order.Closes) > 0 {
			assoc.closes = order.Closes
		}
		if len(order.ClosedBy) > 0 {
			assoc.closedBy = order.ClosedBy
		}
		if len(order.Reconciles) > 0 {
			assoc.reconciles = order.Reconciles
		}
		if assoc.closes != nil || assoc.closedBy != nil || assoc.reconciles != nil {
			m2mFixups = append(m2mFixups, assoc)
		}

		// Record deferred FK fixups
		if order.CloseOrderId != nil && *order.CloseOrderId > 0 {
			deferredCloseOrders = append(deferredCloseOrders, deferredCloseOrderID{
				order:           order,
				oldCloseOrderID: *order.CloseOrderId,
			})
		}

		// Index all trades
		for _, trade := range order.Trades {
			if trade.ID > 0 {
				allTrades[trade.ID] = trade
			}
			if trade.ParentTradeID != nil && *trade.ParentTradeID > 0 {
				deferredParentTrades = append(deferredParentTrades, deferredParentTradeID{
					trade:            trade,
					oldParentTradeID: *trade.ParentTradeID,
				})
			}
		}
		for _, trade := range order.ClosedBy {
			if trade.ID > 0 {
				allTrades[trade.ID] = trade
			}
			if trade.ParentTradeID != nil && *trade.ParentTradeID > 0 {
				deferredParentTrades = append(deferredParentTrades, deferredParentTradeID{
					trade:            trade,
					oldParentTradeID: *trade.ParentTradeID,
				})
			}
		}
		for _, trade := range order.ReconcileTrades {
			if trade.ID > 0 {
				allTrades[trade.ID] = trade
			}
		}
	}

	// --- Pass 0: Save the playground record itself ---
	// Temporarily clear the Orders association so GORM doesn't cascade-create them
	savedOrders := playground.Orders
	playground.Orders = nil
	if err := tx.Save(playground).Error; err != nil {
		playground.Orders = savedOrders
		return fmt.Errorf("RemapAndSavePlayground: failed to save playground: %w", err)
	}
	playground.Orders = savedOrders

	// --- Pass 1: Zero IDs and create orders (topological order) ---
	// Sort: orders without CloseOrderId first, then orders that reference others
	sortedOrders := make([]*OrderRecord, len(orders))
	copy(sortedOrders, orders)
	sort.SliceStable(sortedOrders, func(i, j int) bool {
		iHasClose := sortedOrders[i].CloseOrderId != nil && *sortedOrders[i].CloseOrderId > 0
		jHasClose := sortedOrders[j].CloseOrderId != nil && *sortedOrders[j].CloseOrderId > 0
		if iHasClose != jHasClose {
			return !iHasClose // orders without CloseOrderId come first
		}
		return sortedOrders[i].ID < sortedOrders[j].ID
	})

	for _, order := range sortedOrders {
		oldOrderID := order.ID

		// Strip M2M associations — GORM would try to create them inline
		order.Closes = nil
		order.ClosedBy = nil
		order.Reconciles = nil

		// Nil out CloseOrderId — will be fixed in pass 2
		order.CloseOrderId = nil

		// Zero trade IDs (they'll be auto-assigned via foreignKey association)
		for _, trade := range order.Trades {
			oldTradeID := trade.ID
			trade.ID = 0
			trade.OrderID = nil          // GORM sets this via foreignKey
			trade.ParentTradeID = nil    // Fixed in pass 2
			trade.ParentTrade = nil
			if oldTradeID > 0 {
				// Temporarily store old ID — map will be populated after Create
				state.tradeIDMap[oldTradeID] = 0 // placeholder
			}
		}

		for _, trade := range order.ReconcileTrades {
			oldTradeID := trade.ID
			trade.ID = 0
			trade.ReconcileOrderID = nil
			trade.ParentTradeID = nil
			trade.ParentTrade = nil
			if oldTradeID > 0 {
				state.tradeIDMap[oldTradeID] = 0
			}
		}

		// Zero the order ID so GORM assigns a new one
		order.ID = 0

		// Ensure PlaygroundID is set
		order.PlaygroundID = playground.ID

		// Create the order — GORM auto-assigns order.ID and sets Trades[i].OrderID
		if err := tx.Session(&gorm.Session{FullSaveAssociations: true}).Create(order).Error; err != nil {
			return fmt.Errorf("RemapAndSavePlayground: failed to create order (old ID %d): %w", oldOrderID, err)
		}

		state.orderIDMap[oldOrderID] = order.ID

		// Record new trade IDs
		for _, trade := range order.Trades {
			// Find which old ID this trade had by matching pointer identity
			for oldID, t := range allTrades {
				if t == trade {
					state.tradeIDMap[oldID] = trade.ID
					break
				}
			}
		}
		for _, trade := range order.ReconcileTrades {
			for oldID, t := range allTrades {
				if t == trade {
					state.tradeIDMap[oldID] = trade.ID
					break
				}
			}
		}
	}

	// --- Pass 2: Fix deferred foreign keys ---
	for _, d := range deferredCloseOrders {
		newCloseOrderID, ok := state.orderIDMap[d.oldCloseOrderID]
		if !ok {
			return fmt.Errorf("RemapAndSavePlayground: CloseOrderId references unknown order %d", d.oldCloseOrderID)
		}
		d.order.CloseOrderId = &newCloseOrderID
		if err := tx.Model(d.order).Update("close_order_id", newCloseOrderID).Error; err != nil {
			return fmt.Errorf("RemapAndSavePlayground: failed to update CloseOrderId: %w", err)
		}
	}

	for _, d := range deferredParentTrades {
		newParentTradeID, ok := state.tradeIDMap[d.oldParentTradeID]
		if !ok {
			return fmt.Errorf("RemapAndSavePlayground: ParentTradeID references unknown trade %d", d.oldParentTradeID)
		}
		d.trade.ParentTradeID = &newParentTradeID
		if err := tx.Model(d.trade).Update("parent_trade_id", newParentTradeID).Error; err != nil {
			return fmt.Errorf("RemapAndSavePlayground: failed to update ParentTradeID: %w", err)
		}
	}

	// --- Pass 3: Restore M2M associations ---
	for _, assoc := range m2mFixups {
		if assoc.closes != nil {
			// Closes references other orders — their IDs have been remapped
			assoc.order.Closes = assoc.closes
			if err := tx.Model(assoc.order).Association("Closes").Replace(assoc.closes); err != nil {
				return fmt.Errorf("RemapAndSavePlayground: failed to replace Closes: %w", err)
			}
		}

		if assoc.closedBy != nil {
			// ClosedBy references trades — their IDs have been remapped
			assoc.order.ClosedBy = assoc.closedBy
			if err := tx.Model(assoc.order).Association("ClosedBy").Replace(assoc.closedBy); err != nil {
				return fmt.Errorf("RemapAndSavePlayground: failed to replace ClosedBy: %w", err)
			}
		}

		if assoc.reconciles != nil {
			assoc.order.Reconciles = assoc.reconciles
			if err := tx.Model(assoc.order).Association("Reconciles").Replace(assoc.reconciles); err != nil {
				return fmt.Errorf("RemapAndSavePlayground: failed to replace Reconciles: %w", err)
			}
		}
	}

	// --- Pass 4: Save equity plot records ---
	return saveEquityPlots(tx, playground)
}

// saveEquityPlots persists the in-memory equity plot records for a playground.
func saveEquityPlots(tx *gorm.DB, playground *Playground) error {
	plots := playground.GetEquityPlot()
	if len(plots) == 0 {
		return nil
	}

	records := make([]EquityPlotRecord, 0, len(plots))
	for _, p := range plots {
		records = append(records, EquityPlotRecord{
			PlaygroundID: playground.ID,
			Timestamp:    p.Timestamp,
			Equity:       p.Value,
		})
	}

	if err := tx.CreateInBatches(records, 100).Error; err != nil {
		return fmt.Errorf("saveEquityPlots: failed to save equity plot records: %w", err)
	}

	return nil
}
