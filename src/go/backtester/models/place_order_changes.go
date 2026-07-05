package models

import "gorm.io/gorm"

// OrderSaveIntent is a transaction-free order persistence request: the order
// record to save plus its force-new flag. Intents are staged by the broker
// adapters and persisted by the database layer inside one store-owned
// transaction — no *gorm.DB crosses the package boundary.
type OrderSaveIntent struct {
	Order    *OrderRecord
	ForceNew bool
}

type PlaceOrderChanges struct {
	Commit func(tx *gorm.DB) error
	Info   string
}
