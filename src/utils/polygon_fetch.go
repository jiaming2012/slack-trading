package utils

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func FetchRecursively[T any](url, apiKey string, fetchDataFn eventmodels.FetchDataFunc[T]) (*eventmodels.AggregateResult[T], error) {
	backOff := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second, 32 * time.Second, 64 * time.Second, 128 * time.Second}
	isDone := false
	counter := 0
	var aggregateResult eventmodels.AggregateResult[T]

	for {
		aggregateResult = eventmodels.AggregateResult[T]{}

		if counter > 0 {
			log.Warnf("FetchRecursively: backoff %v", backOff[counter])
			time.Sleep(backOff[counter])
		}

		if counter < len(backOff)-1 {
			counter++
		}

		for {
			resp, err := fetchDataFn(url, apiKey)
			if err != nil {
				return nil, fmt.Errorf("FetchRecursively: failed to fetch stock chart: %w", err)
			}

			aggregateResult.QueryCount += resp.QueryCount
			aggregateResult.ResultsCount += resp.ResultsCount

			aggregateResult.Results = append(aggregateResult.Results, resp.Results...)

			if resp.GetNextURL() == nil {
				isDone = true
				break
			}

			url = *resp.GetNextURL()
			time.Sleep(50 * time.Millisecond)
		}

		if len(aggregateResult.Results) == 0 {
			return nil, fmt.Errorf("FetchRecursively: no results found")
		}

		if isDone {
			break
		}
	}

	return &aggregateResult, nil
}
