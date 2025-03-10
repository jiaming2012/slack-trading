package models

import "github.com/google/uuid"

type UpdateOrderRecordRequest struct {
	Field        string
	OrderRecord  *OrderRecord
	Closes       []*OrderRecord
	Reconciles   []*OrderRecord
	PlaygroundId *uuid.UUID
	ClosedBy     []*TradeRecord
}

