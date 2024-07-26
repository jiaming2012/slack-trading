package utils

import (
	"fmt"
	"os"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func FetchRecursively[T any](url string, fetchDataFn eventmodels.FetchDataFunc[T]) (*eventmodels.AggregateResult[T], error) {
	apiKey := os.Getenv("POLYGON_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("missing POLYGON_API_KEY environment")
	}

	aggregateResult := eventmodels.AggregateResult[T]{}

	for {
		resp, err := fetchDataFn(url, apiKey)
		if err != nil {
			return nil, fmt.Errorf("FetchPolygonStockChart: failed to fetch stock chart: %w", err)
		}

		aggregateResult.QueryCount += resp.QueryCount
		aggregateResult.ResultsCount += resp.ResultsCount

		aggregateResult.Results = append(aggregateResult.Results, resp.Results...)

		if resp.GetNextURL() == nil {
			break
		}

		url = *resp.GetNextURL()
	}

	if len(aggregateResult.Results) == 0 {
		return nil, fmt.Errorf("FetchPolygonStockChart: no results found")
	}

	return &aggregateResult, nil
}
