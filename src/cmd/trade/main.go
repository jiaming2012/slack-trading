package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"slack-trading/src/eventmodels"
	"slack-trading/src/utils"
)

type RunArgs struct {
	UnderlyingSymbol eventmodels.StockSymbol
	BuyToOpenSymbol  eventmodels.OptionSymbol
	SellToOpenSymbol eventmodels.OptionSymbol
	Quantity         int
	GoEnv            string
}

type RunResult struct{}

func PlaceTradeSpread(url string, bearerToken string, underlying eventmodels.StockSymbol, sellToOpenSymbol eventmodels.OptionSymbol, buyToOpenSymbol eventmodels.OptionSymbol, quantity int) error {
	if quantity <= 0 {
		return fmt.Errorf("placeTradeSpread: quantity must be positive")
	}

	quantityStr := strconv.Itoa(quantity)

	client := http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("PlaceTradeSpread: failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Add("class", "multileg")
	q.Add("duration", "GTC")
	q.Add("type", "market")
	q.Add("symbol", string(underlying))
	q.Add("option_symbol[0]", string(buyToOpenSymbol))
	q.Add("quantity[0]", quantityStr)
	q.Add("side[0]", "buy_to_open")
	q.Add("option_symbol[1]", string(sellToOpenSymbol))
	q.Add("quantity[1]", quantityStr)
	q.Add("side[1]", "sell_to_open")

	req.URL.RawQuery = q.Encode()
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", bearerToken))

	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("PlaceTradeSpread: failed to place trade: %w", err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("PlaceTradeSpread: failed to place trade, http code %v", res.Status)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return fmt.Errorf("PlaceTradeSpread: failed to decode response: %w", err)
	}

	if e, found := response["errors"]; found {
		return fmt.Errorf("PlaceTradeSpread: failed to place trade: %v", e)
	}

	return nil
}

var runCmd = &cobra.Command{
	Use:   "go run src/cmd/trade/main.go --underlying spx --side sell --quantity 2 --sell-to-open SPXW240607C05305000 --buy-to-open SPXW240607C05350000",
	Short: "Place a trade and log the result to google sheets",
	Run: func(cmd *cobra.Command, args []string) {
		goEnv, err := cmd.Flags().GetString("go-env")
		if err != nil {
			log.Fatalf("error getting go-env: %v", err)
		}

		if goEnv == "production" {
			fmt.Printf("Are you sure you want to run in production mode? (yes/no): ")
			var response string
			fmt.Scanln(&response)
			if response != "yes" {
				log.Fatalf("exiting")
			}
		}

		underlying, err := cmd.Flags().GetString("underlying")
		if err != nil {
			log.Fatalf("error getting underlying: %v", err)
		}

		buyToOpen, err := cmd.Flags().GetString("buy-to-open")
		if err != nil {
			log.Fatalf("error getting buy-to-open: %v", err)
		}

		sellToOpen, err := cmd.Flags().GetString("sell-to-open")
		if err != nil {
			log.Fatalf("error getting sell-to-open: %v", err)
		}

		quantity, err := cmd.Flags().GetInt("quantity")
		if err != nil {
			log.Fatalf("error getting quantity: %v", err)
		}

		if result, err := Run(RunArgs{
			UnderlyingSymbol: eventmodels.StockSymbol(underlying),
			BuyToOpenSymbol:  eventmodels.OptionSymbol(buyToOpen),
			SellToOpenSymbol: eventmodels.OptionSymbol(sellToOpen),
			Quantity:         quantity,
			GoEnv:            goEnv,
		}); err != nil {
			log.Errorf("Error: %v", err)
		} else {
			log.Infof("Success: %v", result)
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

	bearerToken := os.Getenv("TRADIER_TRADES_BEARER_TOKEN")
	if bearerToken == "" {
		log.Fatalf("missing TRADIER_TRADES_BEARER_TOKEN environment variable")
	}

	tradierTradesURLTemplate := os.Getenv("TRADIER_TRADES_URL_TEMPLATE")
	if tradierTradesURLTemplate == "" {
		log.Fatalf("missing TRADIER_TRADES_URL_TEMPLATE environment variable")
	}

	url := fmt.Sprintf(tradierTradesURLTemplate, accountID)

	if err := PlaceTradeSpread(url, bearerToken, args.UnderlyingSymbol, args.BuyToOpenSymbol, args.SellToOpenSymbol, args.Quantity); err != nil {
		return RunResult{}, fmt.Errorf("error placing long spread trade: %v", err)
	}

	return RunResult{}, nil
}

func main() {
	runCmd.PersistentFlags().String("go-env", "development", "The go environment to run the command in.")
	runCmd.PersistentFlags().String("underlying", "", "The underlying symbol of the spread.")
	runCmd.PersistentFlags().String("buy-to-open", "", "The symbol to buy to open.")
	runCmd.PersistentFlags().String("sell-to-open", "", "The symbol to sell to open.")
	runCmd.PersistentFlags().Int("quantity", 0, "The quantity of the spread to place the trade on.")

	runCmd.MarkPersistentFlagRequired("underlying")
	runCmd.MarkPersistentFlagRequired("buy-to-open")
	runCmd.MarkPersistentFlagRequired("sell-to-open")
	runCmd.MarkPersistentFlagRequired("quantity")

	runCmd.Execute()
}
