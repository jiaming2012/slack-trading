package models

import (
	"time"

	"github.com/google/uuid"
)

type ExecutionFillRequest struct {
	PlaygroundId uuid.UUID
	Price        float64
	Quantity     float64
	Time         time.Time
}
