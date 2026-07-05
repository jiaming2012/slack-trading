package models

import "github.com/google/uuid"

type EventStreamID uuid.UUID

func (e EventStreamID) String() string {
	return uuid.UUID(e).String()
}
