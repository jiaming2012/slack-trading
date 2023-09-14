package eventproducers

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func SetResponse[T any](obj *T, w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)

	if err := json.NewEncoder(w).Encode(obj); err != nil {
		return fmt.Errorf("SetResponse: encode: %w", err)
	}

	return nil
}

func SetErrorResponse(errType string, statusCode int, err error, w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	resp := NewErrorResponse(errType, err.Error())
	if encodeErr := json.NewEncoder(w).Encode(resp); encodeErr != nil {
		return encodeErr
	}

	return nil
}
