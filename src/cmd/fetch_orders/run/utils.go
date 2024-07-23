package run

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path"
	"time"

	"github.com/gocarina/gocsv"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func ExportToCsv(inDir string, results []*eventmodels.OptionOrderSpreadResult) (string, error) {
	now := time.Now()
	outFilePath := path.Join(inDir, fmt.Sprintf("backtest_%s.csv", now.Format("2006-01-02_15-04-05")))

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
