package eventmodels

import (
	"sync"

	log "github.com/sirupsen/logrus"
)

type FIFOQueue[T any] struct {
	caller  string
	queue   chan T
	wg      *sync.WaitGroup
	counter uint
	mutex   *sync.Mutex
}

func NewFIFOQueue[T any](caller string, size int) *FIFOQueue[T] {
	return &FIFOQueue[T]{
		queue:   make(chan T, size),
		wg:      &sync.WaitGroup{},
		counter: 0,
		mutex:   &sync.Mutex{},
		caller:  caller,
	}
}

func (q *FIFOQueue[T]) Enqueue(item T) {
	q.mutex.Lock()
	q.counter++
	counter := q.counter
	q.mutex.Unlock()

	log.Tracef("%v (%p): Enqueueing item: %v, count=%v", q.caller, q, item, counter)
	q.wg.Add(1)
	q.queue <- item
}

func (q *FIFOQueue[T]) Dequeue() (T, bool) {
	q.mutex.Lock()
	counter := q.counter
	q.mutex.Unlock()

	log.Tracef("%v (%p): Dequeueing item, count=%v", q.caller, q, counter)

	select {
	case item := <-q.queue:
		q.wg.Done()
		log.Tracef("%v (%p): Dequeued item: %v, count=%v", q.caller, q, item, counter)

		q.mutex.Lock()
		q.counter--
		q.mutex.Unlock()

		return item, true
	default:
		var zero T
		log.Tracef("%v (%p): Dequeued item: %v, count=%v", q.caller, q, zero, q.counter)
		return zero, false
	}
}

func (q *FIFOQueue[T]) Close() {
	q.wg.Wait()
	close(q.queue)
}
