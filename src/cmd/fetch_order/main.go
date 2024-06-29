package main

import (
	"encoding/json"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/eventservices"
	"github.com/jiaming2012/slack-trading/src/utils"
)

type RunArgs struct {
	OrderID int
	GoEnv   string
}

type RunResult struct {
	Order *eventmodels.OptionOrderSpreadResult
}

var runCmd = &cobra.Command{
	Use:   "go run src/cmd/fetch_order/main.go --orderID 12890162",
	Short: "Fetch the results of a trade by order ID",
	Run: func(cmd *cobra.Command, args []string) {
		goEnv, err := cmd.Flags().GetString("go-env")
		if err != nil {
			log.Fatalf("error getting go-env: %v", err)
		}

		orderID, err := cmd.Flags().GetInt("orderID")
		if err != nil {
			log.Fatalf("error getting orderID: %v", err)
		}

		if result, err := Run(RunArgs{
			OrderID: orderID,
			GoEnv:   goEnv,
		}); err != nil {
			log.Errorf("Error: %v", err)
		} else {
			// Assuming `result.Order` is the struct you want to pretty print
			orderJSON, err := json.MarshalIndent(result.Order, "", "  ")
			if err != nil {
				log.Errorf("Failed to marshal order: %v", err)
			} else {
				fmt.Println(string(orderJSON))
			}
		}
	},
}

func Run(args RunArgs) (RunResult, error) {
	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		log.Fatalf("missing PROJECTS_DIR environment variable")
	}

	if err := utils.InitEnvironmentVariables(projectsDir, args.GoEnv); err != nil {
		log.Fatalf("error loading environment variables: %v", err)
	}

	accountID := os.Getenv("TRADIER_TRADES_ACCOUNT_ID")
	if accountID == "" {
		log.Fatalf("missing TRADIER_TRADES_ACCOUNT_ID environment variable")
	}

	bearerToken := os.Getenv("TRADIER_BEARER_TOKEN")
	if bearerToken == "" {
		log.Fatalf("missing TRADIER_BEARER_TOKEN environment variable")
	}

	tradesBearerToken := os.Getenv("TRADIER_TRADES_BEARER_TOKEN")
	if tradesBearerToken == "" {
		log.Fatalf("missing TRADIER_TRADES_BEARER_TOKEN environment variable")
	}

	quotesHistoryURL := os.Getenv("TRAIER_QUOTES_HISTORY_URL")
	if quotesHistoryURL == "" {
		log.Fatalf("missing TRAIER_QUOTES_HISTORY_URL environment variable")
	}

	tradierTradesURLTemplate := os.Getenv("TRADIER_TRADES_URL_TEMPLATE")
	if tradierTradesURLTemplate == "" {
		log.Fatalf("missing TRADIER_TRADES_URL_TEMPLATE environment variable")
	}

	ordersUrl := fmt.Sprintf(tradierTradesURLTemplate, accountID)

	// move to run
	order, err := eventservices.FetchTradierOrder(ordersUrl, tradesBearerToken, args.OrderID)
	if err != nil {
		return RunResult{}, fmt.Errorf("error fetching tradier order: %v", err)
	}

	option, err := utils.ParseOptionTicker(order.Order.Leg[0].OptionSymbol)
	if err != nil {
		return RunResult{}, fmt.Errorf("error parsing option ticker: %v", err)
	}

	quote, err := eventservices.FetchTradierQuotes(quotesHistoryURL, bearerToken, eventmodels.StockSymbol(order.Order.Symbol), option.Expiration)
	if err != nil {
		return RunResult{}, fmt.Errorf("error fetching tradier quotes: %v", err)
	}

	fmt.Printf("quote: %v\n", quote)

	// resultOrder, err := utils.CalculateOptionOrderSpreadResult(order, )
	// if err != nil {
	// 	return RunResult{}, fmt.Errorf("error calculating option order spread result: %v", err)
	// }

	return RunResult{Order: nil}, nil
}

func main() {
	runCmd.PersistentFlags().String("go-env", "development", "The go environment to run the command in.")
	runCmd.PersistentFlags().Int("orderID", 0, "The tradier order id.")

	runCmd.MarkPersistentFlagRequired("orderID")

	runCmd.Execute()
}
