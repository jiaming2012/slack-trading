package models

type PlaceOrderChanges struct {
	Commit         func() error
	AdditionalInfo string
}
