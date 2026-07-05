package models

import "sync"

type StreamParameter struct {
	StreamName StreamName
	Mutex      *sync.Mutex
}
