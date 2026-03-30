package models

import (
	"context"
	"fmt"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/go/eventmodels"
	"github.com/jiaming2012/slack-trading/src/go/eventproducers"
	"github.com/jiaming2012/slack-trading/src/go/eventservices"
)

// ESDBSignalRepository implements ISignalRepository with write-through
// persistence to EventStoreDB. All signals are stored in the single global
// "trade-signals" stream (per D-01). Reads fetch all events from the stream
// and filter in Go.
type ESDBSignalRepository struct {
	esdbProducer *eventproducers.EsdbProducer
	mutex        sync.Mutex
}

func NewESDBSignalRepository(esdbProducer *eventproducers.EsdbProducer) *ESDBSignalRepository {
	return &ESDBSignalRepository{
		esdbProducer: esdbProducer,
	}
}

func (r *ESDBSignalRepository) Write(signal *eventmodels.TradeSignal) error {
	if signal == nil {
		return fmt.Errorf("cannot write nil signal")
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	if err := r.esdbProducer.Save(context.Background(), signal); err != nil {
		return fmt.Errorf("ESDBSignalRepository.Write: failed to save signal to ESDB: %w", err)
	}

	return nil
}

func (r *ESDBSignalRepository) ReadPending(upTo time.Time) []*eventmodels.TradeSignal {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	signals, err := r.fetchAllFromESDB()
	if err != nil {
		log.Errorf("ESDBSignalRepository.ReadPending: failed to fetch signals from ESDB: %v", err)
		return []*eventmodels.TradeSignal{}
	}

	var result []*eventmodels.TradeSignal
	for _, s := range signals {
		if !s.Timestamp.After(upTo) {
			result = append(result, s)
		}
	}

	if result == nil {
		return []*eventmodels.TradeSignal{}
	}
	return result
}

func (r *ESDBSignalRepository) GetAll() []*eventmodels.TradeSignal {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	signals, err := r.fetchAllFromESDB()
	if err != nil {
		log.Errorf("ESDBSignalRepository.GetAll: failed to fetch signals from ESDB: %v", err)
		return []*eventmodels.TradeSignal{}
	}

	return signals
}

func (r *ESDBSignalRepository) fetchAllFromESDB() ([]*eventmodels.TradeSignal, error) {
	ctx := context.Background()
	client := r.esdbProducer.GetClient()

	signals, err := eventservices.FetchAll[*eventmodels.TradeSignal](ctx, client, &eventmodels.TradeSignal{})
	if err != nil {
		return nil, fmt.Errorf("fetchAllFromESDB: %w", err)
	}

	return signals, nil
}
