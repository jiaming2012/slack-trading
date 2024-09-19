package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/jiaming2012/slack-trading/src/cmd/fetch_orders/run"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/eventservices"
	"github.com/jiaming2012/slack-trading/src/utils"
)

type RunArgs struct {
	OrderIDs []int
	GoEnv    string
	OutDir   *string
}

type RunResult struct {
	Orders []*eventmodels.OptionOrderSpreadResult
}

var runCmd = &cobra.Command{
	Use:   "go run src/cmd/fetch_orders/main.go --orderIDs 12890162,12848807",
	Short: "Fetch results of multiple trades by order ID",
	Run: func(cmd *cobra.Command, args []string) {
		goEnv, err := cmd.Flags().GetString("go-env")
		if err != nil {
			log.Fatalf("error getting go-env: %v", err)
		}

		outDir, err := cmd.Flags().GetString("outDir")
		if err != nil {
			log.Fatalf("error getting outDir: %v", err)
		}

		orderIDs, err := cmd.Flags().GetIntSlice("orderIDs")
		if err != nil {
			log.Fatalf("error getting orderID: %v", err)
		}

		if result, err := Run(RunArgs{
			OrderIDs: orderIDs,
			GoEnv:    goEnv,
		}); err != nil {
			log.Errorf("Error: %v", err)
		} else {
			if outDir == "" {
				orderJSON, err := json.MarshalIndent(result.Orders, "", "  ")
				if err != nil {
					log.Errorf("Failed to marshal order: %v", err)
				} else {
					fmt.Println(string(orderJSON))
				}
			} else {
				csvPath, err := run.ExportToCsv(outDir, result.Orders, "fetch_orders")
				if err != nil {
					log.Errorf("Failed to export to CSV: %v", err)
				} else {
					fmt.Println("CSV file written to: ", csvPath)
				}
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

	polygonAPIKey := os.Getenv("POLYGON_API_KEY")
	if polygonAPIKey == "" {
		log.Fatalf("missing POLYGON_API_KEY environment variable")
	}

	ordersUrl := fmt.Sprintf(tradierTradesURLTemplate, accountID)

	var resultOrders []*eventmodels.OptionOrderSpreadResult
	now := time.Now()

	var tradierOrders []*eventmodels.TradierOrder
	indexOrders := make(map[uint]*eventmodels.TradierOrder)
	indexSymbols := make(map[eventmodels.StockSymbol]struct{})
	var minExpiration, maxExpiration time.Time
	var minCreateDate, maxCreateDate time.Time
	for _, orderID := range args.OrderIDs {
		orderDTO, err := eventservices.FetchTradierOrder(ordersUrl, tradesBearerToken, orderID)
		if err != nil {
			return RunResult{}, fmt.Errorf("error fetching tradier order: %v", err)
		}

		order, err := orderDTO.Order.ToTradierOrder()
		if err != nil {
			return RunResult{}, fmt.Errorf("error converting tradier order: %v", err)
		}

		if order.Status != "filled" {
			continue
		}

		if order.Symbol == "NDX" {
			indexOrders[uint(orderID)] = order
			indexSymbols[eventmodels.StockSymbol(order.Symbol)] = struct{}{}
		}

		option, err := eventmodels.NewOptionSymbolComponents(order.Leg[0].OptionSymbol)
		if err != nil {
			return RunResult{}, fmt.Errorf("1: error parsing option ticker: %v", err)
		}

		if minExpiration.IsZero() || option.Expiration.Before(minExpiration) {
			minExpiration = option.Expiration
		}

		if maxExpiration.IsZero() || option.Expiration.After(maxExpiration) {
			maxExpiration = option.Expiration
		}

		if minCreateDate.IsZero() || order.CreateDate.Before(minCreateDate) {
			minCreateDate = order.CreateDate
		}

		if maxCreateDate.IsZero() || order.CreateDate.After(maxCreateDate) {
			maxCreateDate = order.CreateDate
		}

		tradierOrders = append(tradierOrders, order)
	}

	indexStartDate := eventmodels.GetMinTime(minCreateDate, minExpiration)
	indexEndDate := eventmodels.GetMaxTime(maxCreateDate, maxExpiration)

	indexTicksMap := make(map[eventmodels.StockSymbol][]eventmodels.PolygonCandleDTO)
	for symbol := range indexSymbols {
		log.Infof("Fetching index ticks for symbol: %v", symbol)

		resp, err := eventservices.FetchPolygonIndexChart(symbol, 15, "minute", indexStartDate, indexEndDate, polygonAPIKey)
		if err != nil {
			return RunResult{}, fmt.Errorf("fetchCandles: failed to fetch index ticks for expiration: %v", err)
		}

		indexTicksMap[symbol] = resp.Results
	}

	for _, order := range tradierOrders {
		option, err := eventmodels.NewOptionSymbolComponents(order.Leg[0].OptionSymbol)
		if err != nil {
			return RunResult{}, fmt.Errorf("2: error parsing option ticker: %v", err)
		}

		if option.Expiration.After(now) {
			continue
		}

		var candlesDTO []*eventmodels.CandleDTO

		if _, found := indexOrders[order.ID]; found {
			for _, c := range indexTicksMap[eventmodels.StockSymbol(order.Symbol)] {
				candle, err := c.ToCandleDTO()
				if err != nil {
					return RunResult{}, fmt.Errorf("fetchCandles: failed to convert candle: %v", err)
				}

				candlesDTO = append(candlesDTO, candle)
			}
		} else {
			resp, err := eventservices.FetchPolygonStockChart(eventmodels.StockSymbol(order.Symbol), 1, "minute", order.CreateDate.Add(-5*time.Minute), order.CreateDate, polygonAPIKey)
			if err != nil {
				return RunResult{}, fmt.Errorf("fetchCandles: failed to fetch order.CreatedAt on stock chart: %v", err)
			}

			for _, c := range resp.Results {
				candle, err := c.ToCandleDTO()
				if err != nil {
					return RunResult{}, fmt.Errorf("fetchCandles: failed to convert candle: %v", err)
				}

				candlesDTO = append(candlesDTO, candle)
			}

			resp, err = eventservices.FetchPolygonStockChart(eventmodels.StockSymbol(order.Symbol), 1, "minute", option.Expiration.Add(-5*time.Minute), option.Expiration, polygonAPIKey)
			if err != nil {
				return RunResult{}, fmt.Errorf("fetchCandles: failed to fetch option expiration on stock chart: %v", err)
			}

			for _, c := range resp.Results {
				candle, err := c.ToCandleDTO()
				if err != nil {
					return RunResult{}, fmt.Errorf("fetchCandles: failed to convert candle: %v", err)
				}

				candlesDTO = append(candlesDTO, candle)
			}
		}

		optionMultiplier := 100.0

		resultOrder, err := utils.CalculateOptionOrderSpreadResult(eventmodels.OptionSpreadAnalysisRequest{
			ID:         order.ID,
			Underlying: eventmodels.StockSymbol(option.Underlying),
			Tag:        order.Tag,
			Leg1: eventmodels.OptionSpreadLeg{
				ID:           order.Leg[0].ID,
				Symbol:       order.Leg[0].OptionSymbol,
				Side:         order.Leg[0].Side,
				Quantity:     order.Leg[0].Quantity,
				AvgFillPrice: order.Leg[0].AvgFillPrice,
			},
			Leg2: eventmodels.OptionSpreadLeg{
				ID:           order.Leg[1].ID,
				Symbol:       order.Leg[1].OptionSymbol,
				Side:         order.Leg[1].Side,
				Quantity:     order.Leg[1].Quantity,
				AvgFillPrice: order.Leg[1].AvgFillPrice,
			},
			CreateDate:    order.CreateDate,
			AvgFillPrice:  order.AvgFillPrice,
			ExecutionType: "market",
		}, candlesDTO, optionMultiplier)

		if err != nil {
			return RunResult{}, fmt.Errorf("error calculating option order spread result: %v", err)
		}

		resultOrders = append(resultOrders, resultOrder)
	}

	return RunResult{Orders: resultOrders}, nil
}

func main() {
	runCmd.PersistentFlags().String("go-env", "development", "The go environment to run the command in.")
	runCmd.PersistentFlags().IntSlice("orderIDs", []int{}, "The tradier order id.")
	runCmd.PersistentFlags().String("outDir", "", "The directory to write the output to.")

	runCmd.MarkPersistentFlagRequired("orderID")

	runCmd.Execute()
}
