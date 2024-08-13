package eventconsumers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/eventpubsub"
)

type TradierOrdersMonitoringWorker struct {
	wg                *sync.WaitGroup
	orders            eventmodels.TradierOrderDataStore
	brokerURL         string
	brokerBearerToken string
}

func (w *TradierOrdersMonitoringWorker) GetOrAddOrder(order *eventmodels.TradierOrder) (*eventmodels.TradierOrder, *eventmodels.TradierOrderCreateEvent) {
	if order, ok := w.orders[order.ID]; ok {
		return order, nil
	}

	w.orders.Add(order)

	return order, &eventmodels.TradierOrderCreateEvent{
		Order: order,
	}
}

func (w *TradierOrdersMonitoringWorker) fetchOrders() ([]*eventmodels.TradierOrderDTO, error) {
	client := http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest(http.MethodGet, w.brokerURL, nil)
	if err != nil {
		return nil, fmt.Errorf("TradierOrdersMonitoringWorker:fetchOrders(): failed to create request: %w", err)
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", w.brokerBearerToken))

	log.Debugf("fetching orders from %s", req.URL.String())

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("TradierOrdersMonitoringWorker:fetchOrders(): failed to fetch option prices: %w", err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TradierOrdersMonitoringWorker:fetchOrders(): failed to fetch option prices: %s", res.Status)
	}

	var ordersDTO eventmodels.TradierOrdersDTO
	if err := json.NewDecoder(res.Body).Decode(&ordersDTO); err != nil {
		return nil, fmt.Errorf("TradierOrdersMonitoringWorker:fetchOrders(): failed to decode json: %w", err)
	}

	orders, err := ordersDTO.Parse()
	if err != nil {
		return nil, fmt.Errorf("TradierOrdersMonitoringWorker:fetchOrders(): failed to parse orders: %w", err)
	}

	return orders, nil
}

func (w *TradierOrdersMonitoringWorker) CheckForDelete(ordersDTO []*eventmodels.TradierOrderDTO) []uint {
	result := []uint{}

	for orderID := range w.orders {
		found := false
		for _, orderDTO := range ordersDTO {
			if orderDTO.ID == orderID {
				found = true
				break
			}
		}

		if !found {
			result = append(result, orderID)
		}
	}

	return result
}

func (w *TradierOrdersMonitoringWorker) CheckForCreateOrUpdate(ordersDTO []*eventmodels.TradierOrderDTO) ([]*eventmodels.TradierOrderCreateEvent, []*eventmodels.TradierOrderUpdateEvent) {
	var createOrderEvents []*eventmodels.TradierOrderCreateEvent
	var updateOrderEvents []*eventmodels.TradierOrderUpdateEvent

	for _, orderDTO := range ordersDTO {
		newOrder, err := orderDTO.ToTradierOrder()
		if err != nil {
			log.Errorf("TradierOrdersMonitoringWorker.CheckForCreateOrUpdate: failed to convert order DTO to order: %v", err)
			continue
		}

		_, createOrderEvent := w.GetOrAddOrder(newOrder)
		if createOrderEvent != nil {
			createOrderEvents = append(createOrderEvents, createOrderEvent)
		} else {
			updates := w.orders.Update(newOrder)
			if len(updates) > 0 {
				updateOrderEvents = append(updateOrderEvents, updates...)
			}
		}
	}

	return createOrderEvents, updateOrderEvents
}

func (w *TradierOrdersMonitoringWorker) Start(ctx context.Context) {
	w.wg.Add(1)

	timer := time.NewTicker(5 * time.Second)

	go func() {
		defer w.wg.Done()
		for {
			select {
			case <-ctx.Done():
				log.Info("stopping TradierOrdersMonitoringWorker consumer")
				return
			case <-timer.C:
				ordersDTO, err := w.fetchOrders()
				if err != nil {
					log.Errorf("TradierOrdersMonitoringWorker.Start: failed to fetch orders: %v", err)
					continue
				}

				// check for delete
				orderIDs := w.CheckForDelete(ordersDTO)
				for _, orderID := range orderIDs {
					w.orders.Delete(orderID)
					eventpubsub.PublishEvent("TradierOrdersMonitoringWorker", eventmodels.TradierOrderDeleteEventName, &eventmodels.TradierOrderDeleteEvent{
						OrderID: orderID,
					})
				}

				// check for add or update
				createOrderEvents, updateEvents := w.CheckForCreateOrUpdate(ordersDTO)
				for _, orderEvent := range createOrderEvents {
					eventpubsub.PublishEvent("TradierOrdersMonitoringWorker", eventmodels.TradierOrderCreateEventName, orderEvent)
				}

				for _, updateEvent := range updateEvents {
					eventpubsub.PublishEvent("TradierOrdersMonitoringWorker", eventmodels.TradierOrderUpdateEventName, updateEvent)
				}
			}
		}
	}()
}

func NewTradierOrdersMonitoringWorker(wg *sync.WaitGroup, brokerURL, brokerBearerToken string) *TradierOrdersMonitoringWorker {
	return &TradierOrdersMonitoringWorker{
		wg:                wg,
		orders:            make(map[uint]*eventmodels.TradierOrder),
		brokerURL:         brokerURL,
		brokerBearerToken: brokerBearerToken,
	}
}
