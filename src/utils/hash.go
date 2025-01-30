package utils

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"fmt"
)

func HashStruct(v interface{}) (string, error) {
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)

	// Serialize the struct
	if err := encoder.Encode(v); err != nil {
		return "", err
	}

	// Compute SHA-256 hash
	hash := sha256.Sum256(buf.Bytes())
	return fmt.Sprintf("%x", hash), nil
}
