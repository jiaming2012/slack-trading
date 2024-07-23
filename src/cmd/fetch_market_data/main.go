package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/jiaming2012/slack-trading/src/cmd/fetch_market_data/run"
	plot_candlestick "github.com/jiaming2012/slack-trading/src/cmd/stats/plot_candlestick/run"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/eventservices"
	"github.com/jiaming2012/slack-trading/src/utils"
)

type RunArgs struct {
	GoEnv    string
	OrderIDs []int
}

type RunResult struct {
	ResultURL string
}

func getStrikePrices(o1 *eventmodels.OptionSymbolComponents, o2 *eventmodels.OptionSymbolComponents) (float64, float64) {
	return o1.StrikePrice, o2.StrikePrice
}

var runCmd = &cobra.Command{
	Use:   "go run src/cmd/fetch_order/main.go --orderIDs 12890162,12848807",
	Short: "Fetch results of multiple trades by order ID",
	Run: func(cmd *cobra.Command, args []string) {
		goEnv, err := cmd.Flags().GetString("go-env")
		if err != nil {
			log.Fatalf("error getting go-env: %v", err)
		}

		orderIDs, err := cmd.Flags().GetIntSlice("orderIDs")
		if err != nil {
			log.Fatalf("error getting orderID: %v", err)
		}

		result, err := Run(RunArgs{
			OrderIDs: orderIDs,
			GoEnv:    goEnv,
		})

		if err != nil {
			log.Errorf("Error: %v", err)
		}

		fmt.Printf("Result URL: %s\n", result.ResultURL)
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

	estLocation, err := time.LoadLocation("America/New_York")
	if err != nil {
		return RunResult{}, fmt.Errorf("error loading EST location: %v", err)
	}

	ordersUrl := fmt.Sprintf(tradierTradesURLTemplate, accountID)

	var underlyingSymbol eventmodels.StockSymbol
	var optionLegSymbol1, optionLegSymbol2 string
	var optionType1, optionType2 eventmodels.OptionType
	var orderData eventmodels.OrderData
	var optionOrderData eventmodels.OptionOrderData
	var strikePriceA, strikePriceB float64
	var fromDate, toDate time.Time
	var underlyingCandles []*eventmodels.CandleDTO
	var chartTitle string
	var subplot1, subplot2 string

	for _, orderID := range args.OrderIDs {
		// move to run
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

		option1, err := eventmodels.NewOptionSymbolComponents(order.Leg[0].OptionSymbol)
		if err != nil {
			return RunResult{}, fmt.Errorf("TradierOrder.GetLeg: failed to parse option ticker: %w", err)
		}

		option2, err := eventmodels.NewOptionSymbolComponents(order.Leg[1].OptionSymbol)
		if err != nil {
			return RunResult{}, fmt.Errorf("TradierOrder.GetLeg: failed to parse option ticker: %w", err)
		}

		orderLeg1, orderLeg2, err := order.GetLegs(option1, option2)
		if err != nil {
			return RunResult{}, fmt.Errorf("error parsing option leg 1 ticker: %v", err)
		}

		if orderLeg2.OptionSymbol == option1.Symbol {
			option1, option2 = option2, option1
		}

		underlyingSymbol1 := strings.TrimSpace(option1.Underlying)
		underlyingSymbol2 := strings.TrimSpace(option2.Underlying)
		if underlyingSymbol1 != underlyingSymbol2 {
			return RunResult{}, fmt.Errorf("option 1 underlying '%s' != '%s' option 2 underlying", option1.Underlying, option2.Underlying)
		}

		if underlyingSymbol == "" && underlyingSymbol1 != underlyingSymbol2 {
			return RunResult{}, fmt.Errorf("underlying symbol mismatch: '%s' != '%s'", underlyingSymbol1, underlyingSymbol2)
		}

		if !option1.Expiration.Equal(option2.Expiration) {
			return RunResult{}, fmt.Errorf("option 1 expiration '%s' != '%s' option 2 expiration", option1.Expiration, option2.Expiration)
		}

		if underlyingSymbol == "" {
			underlyingSymbol = eventmodels.StockSymbol(underlyingSymbol1)
		}

		if optionLegSymbol1 == "" {
			optionLegSymbol1 = string(orderLeg1.OptionSymbol)
		}

		if optionLegSymbol2 == "" {
			optionLegSymbol2 = string(orderLeg2.OptionSymbol)
		}

		if optionType1 == "" {
			if option1.OptionType == "C" {
				optionType1 = eventmodels.OptionTypeCall
			} else if option1.OptionType == "P" {
				optionType1 = eventmodels.OptionTypePut
			} else {
				return RunResult{}, fmt.Errorf("invalid option 1 type: %s", option1.OptionType)
			}
		}

		if optionType2 == "" {
			if option2.OptionType == "C" {
				optionType2 = eventmodels.OptionTypeCall
			} else if option2.OptionType == "P" {
				optionType2 = eventmodels.OptionTypePut
			} else {
				return RunResult{}, fmt.Errorf("invalid option 2 type: %s", option2.OptionType)
			}
		}

		strikePriceA, strikePriceB = getStrikePrices(option1, option2)

		orderCreateDateEST := order.CreateDate.In(estLocation)

		chartTitle = fmt.Sprintf("ID #%d, Short %s Strikes: %.2f / %.2f, Expiration: %s, Placed At: %s EST", order.ID, strings.ToUpper(string(optionType1)), strikePriceA, strikePriceB, option1.Expiration.Format("2006-01-02"), orderCreateDateEST.Format("2006-01-02 3:04 PM"))
		subplot1 = fmt.Sprintf("15-Minute %s Candles", underlyingSymbol)
		subplot2 = fmt.Sprintf("%s / %s", optionLegSymbol1, optionLegSymbol2)

		dayOfOpen := order.CreateDate
		dayAfterOpen := order.CreateDate.AddDate(0, 0, 1)
		underlyingPricesNearOpen, err := eventservices.FetchFinancialModelingPrepChart(underlyingSymbol, "1min", dayOfOpen, dayAfterOpen)
		if err != nil {
			return RunResult{}, fmt.Errorf("error fetching underlying prices near open: %v", err)
		}

		underlyingPriceAtOpen, err := run.GetCandleAtDate(order.CreateDate, underlyingPricesNearOpen)
		if err != nil {
			return RunResult{}, fmt.Errorf("error getting underlying price at open: %v", err)
		}

		// Todo: add support for multiple orders
		// Open

		// Append Order Data
		orderData.Date = append(orderData.Date, orderCreateDateEST.Format("2006-01-02 15:04"))
		orderData.Type = append(orderData.Type, "Sell")
		orderData.Price = append(orderData.Price, underlyingPriceAtOpen.Close)

		// Append Option Order Data
		optionOpenPrice := orderLeg1.AvgFillPrice - orderLeg2.AvgFillPrice
		optionOrderData.Date = append(optionOrderData.Date, orderCreateDateEST.Format("2006-01-02 15:04"))
		if optionOpenPrice > 0 {
			optionOrderData.Type = append(optionOrderData.Type, "Sell")
		} else {
			optionOrderData.Type = append(optionOrderData.Type, "Buy")
		}
		optionOrderData.Price = append(optionOrderData.Price, optionOpenPrice)

		// Close
		underlyingPriceNearClose, err := eventservices.FetchFinancialModelingPrepChart(underlyingSymbol, "1min", option1.Expiration, option1.Expiration.AddDate(0, 0, 1))
		if err != nil {
			return RunResult{}, fmt.Errorf("error fetching underlying prices near close: %v", err)
		}

		orderExpirationDateEST := option1.Expiration

		// Append Order Data
		orderData.Date = append(orderData.Date, orderExpirationDateEST.Format("2006-01-02 15:04"))
		orderData.Type = append(orderData.Type, "Buy")
		orderData.Price = append(orderData.Price, underlyingPriceNearClose[0].Close)
		orderData.StrikePriceA = strikePriceA
		orderData.StrikePriceB = strikePriceB

		// Append Option Order Data

		fromDate, toDate = order.CreateDate, option1.Expiration

		underlyingCandles, err := eventservices.FetchFinancialModelingPrepChart(underlyingSymbol, "15min", fromDate, toDate)
		if err != nil {
			return RunResult{}, fmt.Errorf("error fetching underlying candles: %v", err)
		}

		underlyingCandles = utils.ReverseCandlesDTO(underlyingCandles)

		// optionMultiplier := 100.0

		// resultOrder, err := utils.CalculateOptionOrderSpreadResult(order, underlyingCandles, optionMultiplier)
		// if err != nil {
		// 	return RunResult{}, fmt.Errorf("error calculating option order spread result: %v", err)
		// }

		// optionClosePrice := resultOrder.Profit / optionMultiplier
		// optionOrderData.Date = append(optionOrderData.Date, orderExpirationDateEST.Format("2006-01-02 15:04"))
		// if optionClosePrice > 0 {
		// 	optionOrderData.Type = append(optionOrderData.Type, "Buy")
		// } else {
		// 	optionOrderData.Type = append(optionOrderData.Type, "Sell")
		// }
		// optionOrderData.Price = append(optionOrderData.Price, optionClosePrice)

		// Currently only support one order at a time: to support multiple orders, we
		// would have to figure out how to chart multiple strike prices
		break
	}

	log.Infof("Fetching data for underlying symbol %s", underlyingSymbol)

	// if err := run.TransformDateTime(underlyingCandles); err != nil {
	// 	return RunResult{}, fmt.Errorf("error transforming date time: %v", err)
	// }

	log.Infof("Fetched %d underlying candles", len(underlyingCandles))

	log.Infof("Fetching data for option leg 1 symbol %s", optionLegSymbol1)

	// oratsToken := "ORATS_TOKEN"
	// mock1 := eventservices.GenerateFetchOratsDataMock("/Users/jamal/projects/slack-trading/src/cmd/fetch_market_data/mock/sample_option_data_extended_1.csv")
	// optionsData1, err := mock1(underlyingSymbol, oratsToken, fromDate, toDate)
	// if err != nil {
	// 	return RunResult{}, fmt.Errorf("error fetching option leg 1 data: %v", err)
	// }

	// log.Infof("Fetched %d option leg 1 ticks", len(optionsData1))

	var optionCandles1, optionCandles2 []eventmodels.CandleDTO

	thetaDataBaseURL := "http://127.0.0.1:25510"

	optionsDataDTO1, err := eventservices.FetchThetaDataHistOptionOHLC(thetaDataBaseURL, underlyingSymbol, optionType1, toDate, fromDate, toDate, 15*time.Minute, strikePriceA)
	if err != nil {
		return RunResult{}, fmt.Errorf("error fetching option leg 1 data: %v", err)
	}

	optionCandlesDTO1, err := optionsDataDTO1.ConvertToCandles()
	if err != nil {
		return RunResult{}, fmt.Errorf("error converting option leg 1 data to candles: %v", err)
	}

	var previousCandle *eventmodels.CandleDTO
	for _, dto := range optionCandlesDTO1 {
		candle, err := dto.ToCandleDTO()
		if err != nil {
			return RunResult{}, fmt.Errorf("error converting option leg 1 data to candles: %v", err)
		}

		if previousCandle != nil {
			if candle.Volume == 0 {
				candle = *previousCandle
			}
		}

		optionCandles1 = append(optionCandles1, candle)
		previousCandle = &candle
	}

	optionsDataDTO2, err := eventservices.FetchThetaDataHistOptionOHLC(thetaDataBaseURL, underlyingSymbol, optionType2, toDate, fromDate, toDate, 15*time.Minute, strikePriceB)
	if err != nil {
		return RunResult{}, fmt.Errorf("error fetching option leg 2 data: %v", err)
	}

	optionCandlesDTO2, err := optionsDataDTO2.ConvertToCandles()
	if err != nil {
		return RunResult{}, fmt.Errorf("error converting option leg 2 data to candles: %v", err)
	}

	for _, dto := range optionCandlesDTO2 {
		candle, err := dto.ToCandleDTO()
		if err != nil {
			return RunResult{}, fmt.Errorf("error converting option leg 2 data to candles: %v", err)
		}

		if previousCandle != nil {
			if candle.Volume == 0 {
				candle = *previousCandle
			}
		}

		optionCandles2 = append(optionCandles2, candle)
		previousCandle = &candle
	}

	log.Infof("Derived %d option leg 1 candles", len(optionCandles1))

	log.Infof("Fetching data for option leg 2 symbol %s", optionLegSymbol2)

	// mock2 := eventservices.GenerateFetchOratsDataMock("/Users/jamal/projects/slack-trading/src/cmd/fetch_market_data/mock/sample_option_data_extended_2.csv")
	// optionsData2, err := mock2(underlyingSymbol, oratsToken, fromDate, toDate)
	// if err != nil {
	// 	return RunResult{}, fmt.Errorf("error fetching option leg 2 data: %v", err)
	// }

	// log.Infof("Fetched %d option leg 2 ticks", len(optionsData2))

	log.Infof("Derived %d option leg 2 candles", len(optionCandles2))

	spreadCandles := eventmodels.DeriveSpreadCandles(optionCandles2, optionCandles1)

	log.Infof("Derived %d spread candles", len(spreadCandles))

	output, err := plot_candlestick.ExecPlotCandlestick(projectsDir, eventmodels.PlotOrderInputData{
		ChartData: eventmodels.ChartData{
			Title:          chartTitle,
			Sublplot1Title: subplot1,
			Sublplot2Title: subplot2,
			Timeframe:      15,
		},
		CandleData:      eventmodels.CandleDTOs(underlyingCandles).ConvertToCandleData(),
		OrderData:       orderData,
		OptionData:      eventmodels.CandleDTOs(spreadCandles).ConvertToCandleData(),
		OptionOrderData: optionOrderData,
	})

	if err != nil {
		return RunResult{}, fmt.Errorf("error exec plot candlestick: %v", err)
	}

	log.Infof("Output: %s", output)

	return RunResult{}, nil
}

func main() {
	runCmd.PersistentFlags().String("go-env", "development", "The go environment to run the command in.")
	runCmd.PersistentFlags().IntSlice("orderIDs", []int{}, "The tradier order id.")
	runCmd.PersistentFlags().String("outDir", "", "The directory to write the output to.")

	runCmd.MarkPersistentFlagRequired("orderID")

	runCmd.Execute()
}
