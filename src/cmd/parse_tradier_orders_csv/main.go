package main

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/gocarina/gocsv"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type TradierOrderCsvRowDTO struct {
	ID                string  `csv:"ID"`
	ParentID          string  `csv:"Parent ID"`
	Symbol            string  `csv:"Symbol"`
	Side              string  `csv:"Side"`
	QTY               int     `csv:"QTY"`
	Type              string  `csv:"Type"`
	Dur               string  `csv:"Dur"`
	FilledQty         int     `csv:"Filled/Qty."`
	Status            string  `csv:"Status"`
	Class             string  `csv:"Class"`
	Price             float64 `csv:"Price"`
	RemainingQuantity int     `csv:"Remaining Quantity"`
	ReasonDescription string  `csv:"Reason Description"`
	LastFillPrice     float64 `csv:"Last Fill Price"`
	LastFillQuantity  int     `csv:"Last Fill Quantity"`
	CreateDate        string  `csv:"Create Date"`
	TransactionDate   string  `csv:"Transaction Date"`
	ExpirationDate    string  `csv:"Expiration Date"`
	Strike            float64 `csv:"Strike"`
	OptionType        string  `csv:"Option Type"`
	Tag               string  `csv:"Tag"`
}

type RunArgs struct {
	InDir string
}

type RunResults struct {
	OrderIDs []int
}

var runCmd = &cobra.Command{
	Use:   "go run src/cmd/parse_tradier_orders_csv/main.go --inDir orders_2024-04-29_2024-06-28.csv",
	Short: "Returns a list of order IDs from a Tradier orders CSV file.",
	Run: func(cmd *cobra.Command, args []string) {
		inDir, err := cmd.Flags().GetString("inDir")
		if err != nil {
			log.Fatalf("error getting inDir: %v", err)
		}

		if result, err := Run(RunArgs{
			InDir: inDir,
		}); err != nil {
			log.Errorf("Error: %v", err)
		} else {
			var output strings.Builder

			for i := 0; i < len(result.OrderIDs)-1; i++ {
				output.WriteString(fmt.Sprintf("%d,", result.OrderIDs[i]))
			}

			if len(result.OrderIDs) > 0 {
				output.WriteString(fmt.Sprintf("%d", result.OrderIDs[len(result.OrderIDs)-1]))
			} else {
				output.WriteString("No order IDs found.")
			}

			fmt.Println(output.String())
		}
	},
}

func Run(args RunArgs) (RunResults, error) {
	f, err := os.Open(args.InDir)
	if err != nil {
		return RunResults{}, fmt.Errorf("failed to open file: %v", err)
	}
	
	var csvRows []TradierOrderCsvRowDTO
	err = gocsv.UnmarshalFile(f, &csvRows)
	if err != nil {
		return RunResults{}, fmt.Errorf("failed to unmarshal CSV: %v", err)
	}

	orderIdMap := make(map[int]struct{})
	for _, row := range csvRows {
		orderId, err := strconv.Atoi(row.ParentID)
		if err != nil {
			return RunResults{}, fmt.Errorf("failed to convert ParentID %v to int: %v", row.ParentID, err)
		}

		if strings.ToLower(row.Status) == "filled" {
			orderIdMap[orderId] = struct{}{}
		}
	}

	orderIds := make([]int, 0, len(orderIdMap))
	for orderId := range orderIdMap {
		orderIds = append(orderIds, orderId)
	}

	sort.Ints(orderIds)

	return RunResults{
		OrderIDs: orderIds,
	}, nil
}

func main() {
	runCmd.PersistentFlags().String("inDir", "", "The directory to read the input from.")
	runCmd.MarkPersistentFlagRequired("inDir")
	runCmd.Execute()
}
