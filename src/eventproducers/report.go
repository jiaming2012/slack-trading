package eventproducers

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type report struct {
	wg *sync.WaitGroup
}

func (r *report) main() {
	// fmt.Println("executing Report main")
}

func (r *report) Start(ctx context.Context) {
	r.wg.Add(1)
	ticker := time.NewTicker(500 * time.Millisecond)

	go func() {
		defer r.wg.Done()
		for {
			select {
			case <-ctx.Done():
				fmt.Printf("\nstopping Report producer\n")
				return
			case <-ticker.C:
				r.main()
			}
		}
	}()
}

func NewReportClient(wg *sync.WaitGroup) *report {
	return &report{
		wg: wg,
	}
}
