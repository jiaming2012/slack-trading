package models

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/jiaming2012/slack-trading/src/go/models"
)

// InMemorySignalRepository implements ISignalRepository using a sorted slice
// with a cursor index for clock-gated delivery. Thread-safe via sync.Mutex.
type InMemorySignalRepository struct {
	signals []*models.TradeSignal
	cursor  int
	mutex   sync.Mutex
}

func NewInMemorySignalRepository() *InMemorySignalRepository {
	return &InMemorySignalRepository{
		signals: make([]*models.TradeSignal, 0),
		cursor:  0,
	}
}

func (r *InMemorySignalRepository) Write(signal *models.TradeSignal) error {
	if signal == nil {
		return fmt.Errorf("cannot write nil signal")
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Binary search to find insertion point maintaining timestamp sort order
	idx := sort.Search(len(r.signals), func(i int) bool {
		return r.signals[i].Timestamp.After(signal.Timestamp)
	})

	// Insert at idx position
	r.signals = append(r.signals, nil)
	copy(r.signals[idx+1:], r.signals[idx:])
	r.signals[idx] = signal

	// If inserted before cursor, adjust cursor to not skip signals
	if idx < r.cursor {
		r.cursor++
	}

	return nil
}

func (r *InMemorySignalRepository) ReadPending(upTo time.Time) []*models.TradeSignal {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	var result []*models.TradeSignal
	for r.cursor < len(r.signals) && !r.signals[r.cursor].Timestamp.After(upTo) {
		result = append(result, r.signals[r.cursor])
		r.cursor++
	}

	if result == nil {
		return []*models.TradeSignal{}
	}
	return result
}

func (r *InMemorySignalRepository) GetAll() []*models.TradeSignal {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	out := make([]*models.TradeSignal, len(r.signals))
	copy(out, r.signals)
	return out
}
