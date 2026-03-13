package utils

import (
	"fmt"
	"strconv"
	"strings"
)

func ParseLookaheadPeriods(lookaheadPeriods string) ([]int, error) {
	periodsStr := strings.Split(lookaheadPeriods, ",")
	periods := make([]int, 0)

	for _, periodStr := range periodsStr {
		period, err := strconv.Atoi(periodStr)
		if err != nil {
			return nil, fmt.Errorf("error parsing lookahead period: %v", err)
		}

		periods = append(periods, period)
	}

	return periods, nil
}
