package models

import "github.com/google/uuid"

type TradierOrderModifyEvent struct {
	PlaygroundId   uuid.UUID
	TradierOrderID uint
	Field          string
	Old            interface{}
	New            interface{}
	Reason         *string
}
