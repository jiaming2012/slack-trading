package utils

import (
	"strconv"
	"strings"
)

// AtoiSlice converts a comma-separated string to an int slice
func AtoiSlice(s string) ([]int, error) {
	strVals := strings.Split(s, ",")
	intVals := make([]int, len(strVals))
	for i, strVal := range strVals {
		intVal, err := strconv.Atoi(strVal)
		if err != nil {
			return nil, err
		}
		intVals[i] = intVal
	}

	return intVals, nil
}
