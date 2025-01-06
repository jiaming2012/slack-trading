package main

import (
	"fmt"
	"sync"
)

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

type X struct {
	Val int
}

func main() {
	queue := NewFIFOQueue[X](10)

	// Enqueue items
	queue.Enqueue(X{Val: 1})
	queue.Enqueue(X{Val: 2})
	queue.Enqueue(X{Val: 3})
	queue.Enqueue(X{Val: 4})

	// Dequeue items
	var i, maxItems int = 0, 4
	for i = 0; i < maxItems; i++ {
		item, ok := queue.Dequeue()
		if ok {
			fmt.Printf("%d : %d\n", i, item.Val)
		} else {
			fmt.Printf("%d : queue is empty\n", i)
			break
		}
	}

	if i == maxItems {
		fmt.Printf("warning: max items reached\n")
	}

	// Wait for all items to be processed
	queue.Close()
}
