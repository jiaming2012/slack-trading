package main

import (
	"fmt"
	"sync"
	"time"
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

func GetQueryStartEndDates(now time.Time, period time.Duration, loc *time.Location) (time.Time, time.Time) {
	startAfter := now.Add(-23 * time.Hour).In(loc)

	start := startAfter.Truncate(period)

	endAfter := now.Add(24 * time.Hour)

	end := endAfter.Truncate(period)

	return start, end
}

func main() {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		panic(err)
	}

	now := time.Now().Add(-12 * time.Minute).In(loc)
	start, end := GetQueryStartEndDates(now, time.Minute * 30, loc)

	fmt.Printf("start: %s\n", start)	
	fmt.Printf("end: %s\n", end)
}
