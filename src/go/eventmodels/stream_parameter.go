package eventmodels

import "sync"

type StreamParameter struct {
	StreamName StreamName
	Mutex      *sync.Mutex
}
