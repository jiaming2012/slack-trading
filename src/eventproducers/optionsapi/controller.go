package optionsapi

import (
	"net/http"

	"slack-trading/src/eventmodels"
)

type ServeReadOptionChainRequests struct {}

func (s *ServeReadOptionChainRequests) ServeRequest(r *http.Request) (chan interface{}, chan error) {
	result := make(chan interface{})

	go func() {
		options := []*eventmodels.OptionContractV1{}
		result <- options
	}()

	return result, nil
}
