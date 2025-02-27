package models

import "github.com/google/uuid"

type TradierOrderModifyEvent struct {
	PlaygroundId uuid.UUID
	OrderID      uint
	Field        string
	Old          interface{}
	New          interface{}
}
