package eventmodels

import "sync"

type FIFOQueue[T any] struct {
	queue chan T
	wg    sync.WaitGroup
}

func NewFIFOQueue[T any](size int) *FIFOQueue[T] {
	return &FIFOQueue[T]{
		queue: make(chan T, size),
	}
}

func (q *FIFOQueue[T]) Enqueue(item T) {
	q.wg.Add(1)
	q.queue <- item
}

func (q *FIFOQueue[T]) Dequeue() (T, bool) {
	select {
	case item := <-q.queue:
		q.wg.Done()
		return item, true
	default:
		var zero T
		return zero, false
	}
}

func (q *FIFOQueue[T]) Close() {
	q.wg.Wait()
	close(q.queue)
}
