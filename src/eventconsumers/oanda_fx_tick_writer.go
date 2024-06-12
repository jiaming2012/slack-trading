package eventconsumers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"slack-trading/src/eventmodels"
	"slack-trading/src/eventservices"
)

type OandaFxTickWriter struct {
	wg            *sync.WaitGroup
	trackerCli    *TrackerClientV3
	quotesBaseURL string
	bearerToken   string
}

func (w *OandaFxTickWriter) getActiveSymbols() []eventmodels.FxSymbol {
	trackers, done := w.trackerCli.GetSavedEvents()
	done()

	activeFxTrackers := eventservices.GetActiveFxTrackers(trackers)

	symbols := make([]eventmodels.FxSymbol, 0, len(activeFxTrackers))

	for _, tracker := range activeFxTrackers {
		symbols = append(symbols, tracker.StartFxTracker.Symbol)
	}

	return symbols
}

func (w *OandaFxTickWriter) FetchLastCandle(ctx context.Context, symbol eventmodels.FxSymbol) (*eventmodels.OandaFetchQuotesResponseDTO, error) {
	client := http.Client{
		Timeout: 10 * time.Second,
	}

	url := fmt.Sprintf(w.quotesBaseURL, symbol)
	url = fmt.Sprintf("%s?count=1", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("OandaFxTickWriter: Failed to create request: %v", err)
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", w.bearerToken))

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OandaFxTickWriter: Failed to fetch quotes: %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OandaFxTickWriter: Failed to fetch quotes, http code %v", resp.Status)
	}

	var dto eventmodels.OandaFetchQuotesResponseDTO
	if err := json.NewDecoder(resp.Body).Decode(&dto); err != nil {
		return nil, fmt.Errorf("OandaFxTickWriter: Failed to decode json: %v", err)
	}

	return &dto, nil
}

func (w *OandaFxTickWriter) run(ctx context.Context, candlesCh chan<- *eventmodels.FxTick) {
	defer w.wg.Done()

	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			activeSymbols := w.getActiveSymbols()
			for _, s := range activeSymbols {
				resp, err := w.FetchLastCandle(ctx, s)
				if err != nil {
					log.Errorf("OandaFxTickWriter.run: Failed to fetch last candle for %s: %v", err, s)
					continue
				}

				candle, err := resp.GetLastCandle(time.Now().UTC())
				if err != nil {
					log.Errorf("OandaFxTickWriter.run: Failed to get last candle for %s: %v", err, s)
					continue
				}

				candlesCh <- &eventmodels.FxTick{
					Symbol:    s,
					Timestamp: candle.Timestamp,
					Price:     candle.Close,
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

func (w *OandaFxTickWriter) Start(ctx context.Context, candlesCh chan<- *eventmodels.FxTick) {
	w.wg.Add(1)

	log.Debug("Starting OandaFxTickWriter...")

	go w.run(ctx, candlesCh)
}

func NewOandaFxTickWriter(wg *sync.WaitGroup, trackerCli *TrackerClientV3, quotesURL string, bearerToken string) *OandaFxTickWriter {
	return &OandaFxTickWriter{
		wg:            wg,
		trackerCli:    trackerCli,
		quotesBaseURL: quotesURL,
		bearerToken:   bearerToken,
	}
}
