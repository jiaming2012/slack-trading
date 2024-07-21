package eventservices

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func FetchHistOptionOHLC(baseURL string, r eventmodels.ThetaDataHistOptionOHLCRequest) (*eventmodels.ThetaDataResponse, error) {
	client := http.Client{
		Timeout: 15 * time.Second,
	}

	url := fmt.Sprintf("%s/v2/hist/option/ohlc", baseURL)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("FetchHistOptionOHLC: failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Add("root", string(r.Root))
	q.Add("right", string(r.Right))
	q.Add("exp", r.Expiration.Format("20060102"))
	q.Add("start_date", r.StartDate.Format("20060102"))
	q.Add("end_date", r.EndDate.Format("20060102"))
	q.Add("ivl", fmt.Sprintf("%d", (int(r.Interval/time.Minute)*60000)))
	q.Add("strike", fmt.Sprintf("%d", int(r.Strike*1000.0)))

	req.URL.RawQuery = q.Encode()
	req.Header.Add("Accept", "application/json")

	log.Printf("FetchHistOptionOHLC: fetching option ohlc from %v", req.URL.String())

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("FetchHistOptionOHLC: failed to fetch option ohlc: %w", err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("FetchHistOptionOHLC: failed to fetch option ohlc, http code %v", res.Status)
	}

	var dto eventmodels.ThetaDataResponse
	if err := json.NewDecoder(res.Body).Decode(&dto); err != nil {
		return nil, fmt.Errorf("FetchHistOptionOHLC: failed to decode json: %w", err)
	}

	return &dto, nil
}

func FetchThetaDataHistOptionOHLC(baseURL string, root eventmodels.StockSymbol, optionType eventmodels.OptionType, expiration time.Time, startDate time.Time, endDate time.Time, interval time.Duration, strike float64) (eventmodels.ThetaDataResponseDTO, error) {
	var result eventmodels.ThetaDataResponseDTO

	var right string
	switch optionType {
	case eventmodels.OptionTypeCall:
		right = "C"
	case eventmodels.OptionTypePut:
		right = "P"
	default:
		return result, fmt.Errorf("FetchThetaDataOHLC: invalid option type: %v", optionType)
	}

	expirationStr := expiration.Format("20060102")
	startDateStr := startDate.Format("20060102")
	endDateStr := endDate.Format("20060102")

	intervalM := interval / time.Millisecond

	// Validate interval value
	if intervalM < 100 || intervalM > 3600000 {
		return result, fmt.Errorf("FetchThetaDataOHLC: invalid interval value: %d. Must be between 100 and 3600000 milliseconds", intervalM)
	}

	// Convert ivl (time.Duration) to milliseconds
	intervalStr := fmt.Sprintf("%d", intervalM)

	// Convert strike price to 1/10ths of a cent and to integer
	strikeInt := int(strike * 1000)

	// Define the request URL
	url := fmt.Sprintf("%s/v2/hist/option/ohlc?right=%s&exp=%s&start_date=%s&end_date=%s&root=%s&ivl=%s&strike=%d", baseURL, right, expirationStr, startDateStr, endDateStr, root, intervalStr, strikeInt)

	log.Infof("FetchThetaDataOHLC: fetching data from %s", url)

	// Create a new HTTP GET request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return result, fmt.Errorf("FetchThetaDataOHLC: %w", err)
	}

	// Execute the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return result, fmt.Errorf("FetchThetaDataOHLC.Do: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return result, fmt.Errorf("FetchThetaDataOHLC.ReadAll: %w", err)
	}

	// Unmarshal the JSON response into the result struct
	err = json.Unmarshal(body, &result)
	if err != nil {
		return result, fmt.Errorf("FetchThetaDataOHLC.Unmarshal: %w", err)
	}

	return result, nil
}
