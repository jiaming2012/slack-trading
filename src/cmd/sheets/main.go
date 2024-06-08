package main

import (
	"context"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"slack-trading/src/sheets"
	"slack-trading/src/utils"
)

type RunArgs struct {
	SpreadsheetId string
	SheetName     string
	GoEnv         string
	Values        []string
}

type RunResult struct{}

var runCmd = &cobra.Command{
	Use:   "go run src/cmd/sheets/main.go",
	Short: "Add a row to a google sheet",
	Run: func(cmd *cobra.Command, args []string) {
		spreadSheetID, err := cmd.Flags().GetString("spreadsheet-id")
		if err != nil {
			log.Fatalf("error getting spreadSheetID: %v", err)
		}

		sheetName, err := cmd.Flags().GetString("sheet-name")
		if err != nil {
			log.Fatalf("error getting sheetName: %v", err)
		}

		goEnv, err := cmd.Flags().GetString("go-env")
		if err != nil {
			log.Fatalf("error getting go-env: %v", err)
		}

		values, err := cmd.Flags().GetStringArray("values")
		if err != nil {
			log.Fatalf("error getting data: %v", err)
		}

		if result, err := Run(RunArgs{
			SpreadsheetId: spreadSheetID,
			SheetName:     sheetName,
			GoEnv:         goEnv,
			Values:        values,
		}); err != nil {
			log.Errorf("Error: %v", err)
		} else {
			log.Infof("Success: %v", result)
		}
	},
}

func Run(args RunArgs) (RunResult, error) {
	ctx := context.Background()

	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		return RunResult{}, fmt.Errorf("PROJECTS_DIR environment variable is not set")
	}

	if err := utils.InitEnvironmentVariables(projectsDir, args.GoEnv); err != nil {
		return RunResult{}, fmt.Errorf("failed to initialize environment variables: %v", err)
	}

	googleSecurityKeyJsonBase64 := os.Getenv("GOOGLE_SECURITY_KEY_JSON_BASE64")
	if googleSecurityKeyJsonBase64 == "" {
		return RunResult{}, fmt.Errorf("GOOGLE_SECURITY_KEY_JSON_BASE64 environment variable is not set")
	}

	sheetsCli, _, err := sheets.NewClient(ctx, googleSecurityKeyJsonBase64)
	if err != nil {
		return RunResult{}, fmt.Errorf("failed to initialize google sheets: %v", err)
	}

	log.Infof("Appending row(length)= %v", len(args.Values))

	row := []interface{}{}
	for _, d := range args.Values {
		row = append(row, d)
	}

	values := [][]interface{}{row}

	if err := sheets.AppendRows(ctx, sheetsCli, args.SpreadsheetId, args.SheetName, values); err != nil {
		return RunResult{}, fmt.Errorf("failed to append rows: %v", err)
	}

	return RunResult{}, nil
}

func main() {
	runCmd.PersistentFlags().StringP("spreadsheet-id", "s", "", "Google Sheets spreadsheet id. Can be found in the web URL. Example: https://docs.google.com/spreadsheets/d/<spreadsheet-id>/edit")
	runCmd.PersistentFlags().StringP("sheet-name", "n", "", "Google Sheets sheet name")
	runCmd.PersistentFlags().StringP("go-env", "e", "development", "Golang environment")
	runCmd.PersistentFlags().StringArrayP("values", "v", []string{}, "Comma-separated list of values to append to the sheet")

	runCmd.MarkPersistentFlagRequired("spreadsheet-id")
	runCmd.MarkPersistentFlagRequired("sheet-name")

	runCmd.Execute()
}
