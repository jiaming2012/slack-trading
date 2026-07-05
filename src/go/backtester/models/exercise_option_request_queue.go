package models

import (
	"sync"

	"github.com/jiaming2012/slack-trading/src/go/models"
)

type ExerciseOptionRequestQueue struct {
	Requests []*models.ExerciseOptionRequest
	mutex    sync.Mutex
}

func NewExerciseOptionRequestQueue() *ExerciseOptionRequestQueue {
	return &ExerciseOptionRequestQueue{
		Requests: make([]*models.ExerciseOptionRequest, 0),
	}
}

func (q *ExerciseOptionRequestQueue) Enqueue(req *models.ExerciseOptionRequest) {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	q.Requests = append(q.Requests, req)
}

func (q *ExerciseOptionRequestQueue) Drain() []*models.ExerciseOptionRequest {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	drained := q.Requests
	q.Requests = make([]*models.ExerciseOptionRequest, 0)
	return drained
}
