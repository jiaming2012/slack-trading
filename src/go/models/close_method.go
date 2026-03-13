package models

type CloseMethod string

const (
	FIFO CloseMethod = "FIFO"
	LIFO CloseMethod = "LIFO"
)
