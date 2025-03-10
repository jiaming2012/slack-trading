package models

type PlaceOrderChanges struct {
	Commit func() error
	Info   string
}
