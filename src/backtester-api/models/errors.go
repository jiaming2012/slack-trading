package models

import "fmt"

var (
	ErrDbOrderIsNotOpenOrPending = fmt.Errorf("order record is not open or pending")
)
