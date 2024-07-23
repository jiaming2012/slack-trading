package run

import (
	"crypto/sha256"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path"

	"github.com/gocarina/gocsv"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

// hashOptionOrderSpreadResult takes each OrderID and returns its SHA-256 hash as a hex string.
func hashOptionOrderSpreadResult(results []*eventmodels.OptionOrderSpreadResult) string {
	var orderIDBytes []byte
	for _, result := range results {
		// Convert OrderID to byte slice. Since OrderID is a uint, it needs to be converted to a string first.
		orderIDBytes = append(orderIDBytes, []byte(fmt.Sprintf("%d", result.OrderID))...)
	}

	// Compute SHA-256 hash
	hash := sha256.Sum256(orderIDBytes)

	// Convert hash to hex string
	hashHex := fmt.Sprintf("%x", hash)

	return hashHex
}

func ExportToCsv(inDir string, results []*eventmodels.OptionOrderSpreadResult) (string, error) {
	outFilePath := path.Join(inDir, fmt.Sprintf("%s.csv", hashOptionOrderSpreadResult(results)))

	// Create directory if it doesn't exist
	if _, err := os.Stat(inDir); os.IsNotExist(err) {
		if err := os.MkdirAll(inDir, os.ModePerm); err != nil {
			return "", err
		}
	}

	// Open a file for writing
	file, err := os.Create(outFilePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Ensure gocsv can work with custom types if you have any in your struct
	gocsv.SetCSVWriter(func(out io.Writer) *gocsv.SafeCSVWriter {
		writer := csv.NewWriter(out)
		writer.Comma = ',' // Customize comma if needed, default is ','
		return gocsv.NewSafeCSVWriter(writer)
	})

	// Marshal and write the data to the file
	if err := gocsv.MarshalFile(&results, file); err != nil { // Note: &results is a pointer to the slice
		return "", err
	}

	return outFilePath, nil
}
