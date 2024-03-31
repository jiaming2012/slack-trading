package eventproducers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"

	"slack-trading/src/eventdto"
	"slack-trading/src/eventmodels"
	pubsub "slack-trading/src/eventpubsub"
)

type trendSpiderClient struct {
	wg     *sync.WaitGroup
	router *mux.Router
}

func (c *trendSpiderClient) main() {
	// fmt.Println("executing Report main")
}

func (c *trendSpiderClient) decodeSignal(webhook *eventdto.TrendspiderWebhook) (interface{}, error) {
	switch webhook.Header.Signal {
	case "support-break":
		var signal eventdto.SupportBreakSignal
		if err := json.Unmarshal(webhook.Data, &signal); err != nil {
			return nil, fmt.Errorf("failed to unmarshal support-break signal: %w", err)
		}

		return signal, nil
	case "resistance-break":
		var signal eventdto.ResistanceBreakSignal
		if err := json.Unmarshal(webhook.Data, &signal); err != nil {
			return nil, fmt.Errorf("failed to unmarshal support-break signal: %w", err)
		}

		return signal, nil
	case "trendline-break":
		var signal eventdto.TrendlineBreakSignal
		if err := json.Unmarshal(webhook.Data, &signal); err != nil {
			return nil, fmt.Errorf("failed to unmarshal support-break signal: %w", err)
		}

		return signal, nil
	default:
		return nil, fmt.Errorf("trendSpiderClient.decodeSignal: unknown signal: %v", webhook.Header.Signal)
	}
}
func (c *trendSpiderClient) webhookHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		var payload eventdto.TrendspiderWebhook
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			log.Errorf("trendSpiderClient.handleWebhook Decode: %v", err)
			return
		}

		timeframeDuration, err := payload.Header.Timeframe.Validate()
		if err != nil {
			log.Errorf("trendSpiderClient.handleWebhook Validate: %v", err)
			return
		}

		decoded, err := c.decodeSignal(&payload)
		if err != nil {
			log.Errorf("handleWebhook: %v", err)
			return
		}

		switch signal := decoded.(type) {
		case eventdto.SupportBreakSignal:
			price, convErr := strconv.ParseFloat(signal.Price, 64)
			if convErr != nil {
				log.Errorf("trendSpiderClient.handleWebhook::SupportBreakSignal: %v", convErr)
				return
			}

			pubsub.PublishEventResultDeprecated("trendSpiderClient.handleWebhook", eventmodels.SupportBreakSignalEventName, eventmodels.SupportBreakSignal{
				Symbol:           payload.Header.Symbol,
				Timeframe:        timeframeDuration,
				Price:            price,
				PriceActionEvent: payload.Header.PriceActionEvent,
			})
		case eventdto.ResistanceBreakSignal:
			price, convErr := strconv.ParseFloat(signal.Price, 64)
			if convErr != nil {
				log.Errorf("trendSpiderClient.handleWebhook::ResistanceBreakSignal: %v", convErr)
				return
			}

			pubsub.PublishEventResultDeprecated("trendSpiderClient.handleWebhook", eventmodels.ResistanceBreakSignalEventName, eventmodels.ResistanceBreakSignal{
				Symbol:           payload.Header.Symbol,
				Timeframe:        timeframeDuration,
				Price:            price,
				PriceActionEvent: payload.Header.PriceActionEvent,
			})
		case eventdto.TrendlineBreakSignal:
			price, convErr := strconv.ParseFloat(signal.Price, 64)
			if convErr != nil {
				log.Errorf("trendSpiderClient.handleWebhook::TrendlineBreakSignal: %v", convErr)
				return
			}

			pubsub.PublishEventResultDeprecated("trendSpiderClient.handleWebhook", eventmodels.TrendlineBreakSignalEventName, eventmodels.TrendlineBreakSignal{
				Symbol:           payload.Header.Symbol,
				Timeframe:        timeframeDuration,
				Price:            price,
				PriceActionEvent: payload.Header.PriceActionEvent,
			})
		default:
			pubsub.PublishError("trendSpiderClient.handleWebhook", fmt.Errorf("unknown signal type %T", signal))
		}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		log.Errorf("tradeHandler: unsuppored method %s", r.Method)
		fmt.Fprintf(w, "traderHandler: unsupported method")
	}
}

func (c *trendSpiderClient) Start(ctx context.Context) {
	c.wg.Add(1)
	ticker := time.NewTicker(500 * time.Millisecond)

	c.router.HandleFunc("/trendspider", c.webhookHandler)

	go func() {
		defer c.wg.Done()
		for {
			select {
			case <-ctx.Done():
				fmt.Printf("\nstopping Trendspider producer\n")
				return
			case <-ticker.C:
				c.main()
			}
		}
	}()
}

func NewTrendSpiderClient(wg *sync.WaitGroup, router *mux.Router) *trendSpiderClient {
	return &trendSpiderClient{
		wg:     wg,
		router: router,
	}
}
