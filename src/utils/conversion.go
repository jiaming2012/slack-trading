package utils

import (
	"fmt"
	"strconv"
	"strings"
)

// AtoiSlice converts a comma-separated string to an int slice
func AtoiSlice(s string) ([]int, error) {
	strVals := strings.Split(s, ",")
	intVals := make([]int, len(strVals))
	for i, strVal := range strVals {
		strVal = strings.TrimSpace(strVal)
		intVal, err := strconv.Atoi(strVal)
		if err != nil {
			return nil, fmt.Errorf("failed to convert '%s' to int: %v", strVal, err)
		}
		intVals[i] = intVal
	}

	return intVals, nil
}
