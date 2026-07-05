package models

import "gorm.io/gorm"

type PlaceOrderChanges struct {
	Commit func(tx *gorm.DB) error
	Info   string
}
