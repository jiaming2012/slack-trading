package models

import (
	"sync"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type ExerciseOptionRequestQueue struct {
	Requests []*eventmodels.ExerciseOptionRequest
	mutex    sync.Mutex
}

func NewExerciseOptionRequestQueue() *ExerciseOptionRequestQueue {
	return &ExerciseOptionRequestQueue{
		Requests: make([]*eventmodels.ExerciseOptionRequest, 0),
	}
}

func (q *ExerciseOptionRequestQueue) Enqueue(req *eventmodels.ExerciseOptionRequest) {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	q.Requests = append(q.Requests, req)
}

func (q *ExerciseOptionRequestQueue) Drain() []*eventmodels.ExerciseOptionRequest {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	drained := q.Requests
	q.Requests = make([]*eventmodels.ExerciseOptionRequest, 0)
	return drained
}
