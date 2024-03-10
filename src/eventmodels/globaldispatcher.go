package eventmodels

import (
	"fmt"
	"sync"

	"github.com/google/uuid"
)

var dispatcher *GlobalResponseDispatcher

type GlobalResponseDispatchItem struct {
	ResultCh chan interface{}
	ErrCh    chan error
}

type GlobalResponseDispatcher struct {
	mutex    sync.Mutex
	Channels map[uuid.UUID]GlobalResponseDispatchItem
}

func (d *GlobalResponseDispatcher) unregister(uuid uuid.UUID) {
	delete(d.Channels, uuid)
}

// GetChannelAndRemove fetches and channel and removes it
func (d *GlobalResponseDispatcher) GetChannelAndRemove(uuid uuid.UUID) (GlobalResponseDispatchItem, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	globalDispatchItem, found := d.Channels[uuid]
	if !found {
		return GlobalResponseDispatchItem{}, fmt.Errorf("GlobalResponseDispatcher.FetchChannel: lookup failed using uuid %s", uuid)
	}

	d.unregister(uuid)

	return globalDispatchItem, nil
}

func RegisterResultCallback(requestID uuid.UUID) (chan interface{}, chan error) {
	dispatcher.mutex.Lock()
	defer dispatcher.mutex.Unlock()

	resultCh := make(chan interface{})
	errCh := make(chan error)

	dispatcher.Channels[requestID] = GlobalResponseDispatchItem{
		ResultCh: resultCh,
		ErrCh:    errCh,
	}

	return resultCh, errCh
}

func InitializeGlobalDispatcher() *GlobalResponseDispatcher {
	dispatcher = &GlobalResponseDispatcher{}
	dispatcher.Channels = make(map[uuid.UUID]GlobalResponseDispatchItem)
	return dispatcher
}
