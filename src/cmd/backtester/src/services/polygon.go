package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func fetchOptionThetaBulkHistOptionOhlc(baseURL string, r eventmodels.ThetaDataBulkHistOptionOHLCRequest) (*eventmodels.ThetaDataBulkResponse, error) {
	client := http.Client{
		Timeout: 10 * time.Second,
	}

	url := fmt.Sprintf("%s/v2/bulk_hist/option/ohlc", baseURL)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("fetchOptionThetaBulkHistOptionOhlc: failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Add("root", string(r.Root))
	q.Add("exp", r.Expiration.Format("20060102"))
	q.Add("start_date", r.StartDate.Format("20060102"))
	q.Add("end_date", r.EndDate.Format("20060102"))
	q.Add("ivl", fmt.Sprintf("%d", (int(r.Interval/time.Minute)*60000)))

	req.URL.RawQuery = q.Encode()
	req.Header.Add("Accept", "application/json")

	log.Printf("fetchOptionThetaBulkHistOptionOhlc: fetching option ohlc from %v", req.URL.String())

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetchOptionThetaBulkHistOptionOhlc: failed to fetch option ohlc: %w", err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetchOptionThetaBulkHistOptionOhlc: failed to fetch option ohlc, http code %v", res.Status)
	}

	var dto eventmodels.ThetaDataBulkResponse
	if err := json.NewDecoder(res.Body).Decode(&dto); err != nil {
		return nil, fmt.Errorf("fetchOptionThetaBulkHistOptionOhlc: failed to decode json: %w", err)
	}

	return &dto, nil
}
