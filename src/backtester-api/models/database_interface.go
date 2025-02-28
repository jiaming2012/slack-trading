package models

import (
	"github.com/google/uuid"
)

type IDatabase interface {
	SaveOrderRecord(playgroundId uuid.UUID, order *BacktesterOrder, liveAccountType LiveAccountType) error
}
