package models

import (
	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type TradierOrderDataStore map[uint]*eventmodels.TradierOrder

func (o TradierOrderDataStore) Update(order *eventmodels.TradierOrder) []*TradierOrderModifyEvent {
	var updates []*TradierOrderModifyEvent

	if o, ok := o[order.ID]; ok {
		if o.Status != order.Status {
			updates = append(updates, &TradierOrderModifyEvent{
				OrderID: order.ID,
				Field:   "status",
				Old:     o.Status,
				New:     order.Status,
			})

			// creating the update event and doing the update inside the same method makes it harder to save
			// and test the update event in a event source model
			o.Status = order.Status
		}
	}

	return updates
}

func (o TradierOrderDataStore) Add(order *eventmodels.TradierOrder) {
	o[order.ID] = order
	log.Debugf("TradierOrdersMonitoringWorker.Add: added order with ID: %d", order.ID)
}

func (o TradierOrderDataStore) Delete(orderID uint) {
	delete(o, orderID)
	log.Debugf("TradierOrdersMonitoringWorker.Delete: removed order with ID: %d", orderID)
}

func NewTradierOrderDataStore() TradierOrderDataStore {
	return make(map[uint]*eventmodels.TradierOrder)
}
