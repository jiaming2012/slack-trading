package utils

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/go/eventmodels"
)

func FetchRecursively[T any](url, apiKey string, fetchDataFn eventmodels.FetchDataFunc[T]) (*eventmodels.AggregateResult[T], error) {
	const maxRetries = 3
	retryBackoff := []time.Duration{2 * time.Second, 4 * time.Second, 8 * time.Second}

	var aggregateResult eventmodels.AggregateResult[T]
	currentURL := url

	for {
		resp, err := fetchDataFn(currentURL, apiKey)
		if err != nil {
			// Retry transient errors (GOAWAY, connection reset, etc.)
			retried := false
			for attempt := 0; attempt < maxRetries; attempt++ {
				log.Warnf("FetchRecursively: transient error (attempt %d/%d): %v — retrying in %v", attempt+1, maxRetries, err, retryBackoff[attempt])
				time.Sleep(retryBackoff[attempt])
				resp, err = fetchDataFn(currentURL, apiKey)
				if err == nil {
					retried = true
					break
				}
			}
			if !retried {
				return nil, fmt.Errorf("FetchRecursively: failed after %d retries: %w", maxRetries, err)
			}
		}

		aggregateResult.QueryCount += resp.QueryCount
		aggregateResult.ResultsCount += resp.ResultsCount
		aggregateResult.Results = append(aggregateResult.Results, resp.Results...)

		if resp.GetNextURL() == nil {
			break
		}

		currentURL = *resp.GetNextURL()
		time.Sleep(50 * time.Millisecond)
	}

	if len(aggregateResult.Results) == 0 {
		log.Warn("FetchRecursively: no results found")
	}

	return &aggregateResult, nil
}
