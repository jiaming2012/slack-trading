package eventservices

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gocarina/gocsv"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func deriveCandleEndTime(data *eventmodels.OratsOptionData, period time.Duration) time.Time {
	return data.SnapShotEstTime.Truncate(period).Add(period)
}

func ConvertOratsOptionDataToCandlesDTO(data []eventmodels.OratsOptionData, period time.Duration, optionType eventmodels.OptionType) ([]eventmodels.CandleDTO, error) {
	candlesDTO := make([]eventmodels.CandleDTO, 0)

	if optionType != eventmodels.OptionTypeCall && optionType != eventmodels.OptionTypePut {
		return nil, fmt.Errorf("unknown option type: %v", optionType)
	}

	// loc, err := time.LoadLocation("America/New_York")
	// if err != nil {
	// 	return nil, fmt.Errorf("error loading location: %v", err)
	// }

	var candleEndTime time.Time
	for _, d := range data {
		var price float64
		if optionType == eventmodels.OptionTypeCall {
			price = d.CallBidPrice
		} else {
			price = d.PutBidPrice
		}

		if !d.SnapShotEstTime.Before(candleEndTime) {
			// date, err := time.ParseInLocation(time.RFC3339, d.SnapShotEstTime)
			// if err != nil {
			// 	return nil, fmt.Errorf("error parsing date: %v", err)
			// }

			candlesDTO = append(candlesDTO, eventmodels.CandleDTO{
				Open:  price,
				High:  price,
				Low:   price,
				Close: price,
				Date:  d.SnapShotEstTime.Format("2006-01-02 15:04"),
			})

			candleEndTime = deriveCandleEndTime(&d, period)
		} else {
			// Update candle price

			if price > candlesDTO[len(candlesDTO)-1].High {
				candlesDTO[len(candlesDTO)-1].High = price
			}

			if price < candlesDTO[len(candlesDTO)-1].Low {
				candlesDTO[len(candlesDTO)-1].Low = price
			}

			candlesDTO[len(candlesDTO)-1].Close = price
		}
	}

	return candlesDTO, nil
}

func GenerateFetchOratsDataMock(url string) func(ticker eventmodels.StockSymbol, token string, fromDate, toDate time.Time) ([]eventmodels.OratsOptionData, error) {
	f, err := os.Open(url)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}

	data, err := io.ReadAll(f)
	if err != nil {
		log.Fatalf("error reading file: %v", err)
	}

	return func(ticker eventmodels.StockSymbol, token string, fromDate, toDate time.Time) ([]eventmodels.OratsOptionData, error) {
		options, err := ParseCSV(string(data))

		if err != nil {
			return nil, fmt.Errorf("error parsing CSV: %v", err)
		}

		return options, nil
	}
}

// FetchData fetches the data from the URL and parses it into OptionData.
func FetchOratsData(ticker, token string, fromDate, toDate time.Time) ([]eventmodels.OratsOptionData, error) {
	tradeDate := fmt.Sprintf("%s,%s", fromDate.Format("200601021504"), toDate.Format("200601021504"))
	url := fmt.Sprintf("https://api.orats.io/datav2/hist/one-minute/strikes/option?token=%s&ticker=%s&tradeDate=%s", token, ticker, tradeDate)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error making GET request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("non-200 response: %s", string(bodyBytes))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}

	csvData := string(bodyBytes)
	options, err := ParseCSV(csvData)
	if err != nil {
		return nil, fmt.Errorf("error parsing CSV: %v", err)
	}

	return options, nil
}

func ParseCSV(data string) ([]eventmodels.OratsOptionData, error) {
	var out []eventmodels.OratsOptionData
	if err := gocsv.UnmarshalBytes([]byte(data), &out); err != nil {
		return nil, fmt.Errorf("error unmarshalling CSV: %v", err)
	}

	return out, nil
}

// ParseCSV parses CSV data into a slice of OratsOptionData.
func ParseCSVOld(data string) ([]eventmodels.OratsOptionData, error) {
	r := csv.NewReader(strings.NewReader(data))
	r.TrimLeadingSpace = true
	r.FieldsPerRecord = -1 // Allow variable fields per record

	// Read header
	_, err := r.Read()
	if err != nil {
		return nil, err
	}

	var options []eventmodels.OratsOptionData
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		option, err := parseRecord(record)
		if err != nil {
			return nil, err
		}
		options = append(options, option)
	}

	return options, nil
}

// parseRecord parses a single CSV record into an eventmodels.OratsOptionData.
func parseRecord(record []string) (eventmodels.OratsOptionData, error) {
	dte, err := strconv.Atoi(record[3])
	if err != nil {
		return eventmodels.OratsOptionData{}, fmt.Errorf("error parsing DTE: %v", err)
	}

	strike, err := strconv.ParseFloat(record[4], 64)
	if err != nil {
		return eventmodels.OratsOptionData{}, fmt.Errorf("error parsing strike: %v", err)
	}

	stockPrice, err := strconv.ParseFloat(record[5], 64)
	if err != nil {
		return eventmodels.OratsOptionData{}, fmt.Errorf("error parsing stock price: %v", err)
	}

	callVolume, err := strconv.Atoi(record[6])
	if err != nil {
		return eventmodels.OratsOptionData{}, fmt.Errorf("error parsing call volume: %v", err)
	}

	callOpenInterest, err := strconv.Atoi(record[7])
	if err != nil {
		return eventmodels.OratsOptionData{}, fmt.Errorf("error parsing call open interest: %v", err)
	}

	callBidSize, err := strconv.Atoi(record[8])
	if err != nil {
		return eventmodels.OratsOptionData{}, fmt.Errorf("error parsing call bid size: %v", err)
	}

	callAskSize, err := strconv.Atoi(record[9])
	if err != nil {
		return eventmodels.OratsOptionData{}, fmt.Errorf("error parsing call ask size: %v", err)
	}

	putVolume, err := strconv.Atoi(record[10])
	if err != nil {
		return eventmodels.OratsOptionData{}, err
	}

	putOpenInterest, err := strconv.Atoi(record[11])
	if err != nil {
		return eventmodels.OratsOptionData{}, err
	}

	putBidSize, err := strconv.Atoi(record[12])
	if err != nil {
		return eventmodels.OratsOptionData{}, err
	}

	putAskSize, err := strconv.Atoi(record[13])
	if err != nil {
		return eventmodels.OratsOptionData{}, err
	}

	callBidPrice, err := strconv.ParseFloat(record[14], 64)
	if err != nil {
		return eventmodels.OratsOptionData{}, err
	}

	callValue, err := strconv.ParseFloat(record[15], 64)
	if err != nil {
		return eventmodels.OratsOptionData{}, err
	}

	callAskPrice, err := strconv.ParseFloat(record[16], 64)
	if err != nil {
		return eventmodels.OratsOptionData{}, err
	}

	putBidPrice, err := strconv.ParseFloat(record[17], 64)
	if err != nil {
		return eventmodels.OratsOptionData{}, err
	}

	putValue, err := strconv.ParseFloat(record[18], 64)
	if err != nil {
		return eventmodels.OratsOptionData{}, err
	}

	putAskPrice, err := strconv.ParseFloat(record[19], 64)
	if err != nil {
		return eventmodels.OratsOptionData{}, err
	}

	callBidIv, err := strconv.ParseFloat(record[20], 64)
	if err != nil {
		return eventmodels.OratsOptionData{}, err
	}

	callMidIv, err := strconv.ParseFloat(record[21], 64)
	if err != nil {
		return eventmodels.OratsOptionData{}, err
	}

	callAskIv, err := strconv.ParseFloat(record[22], 64)
	if err != nil {
		return eventmodels.OratsOptionData{}, err
	}

	smvVol, err := strconv.ParseFloat(record[23], 64)
	if err != nil {
		return eventmodels.OratsOptionData{}, err
	}

	putBidIv, err := strconv.ParseFloat(record[24], 64)
	if err != nil {
		return eventmodels.OratsOptionData{}, err
	}

	putMidIv, err := strconv.ParseFloat(record[25], 64)
	if err != nil {
		return eventmodels.OratsOptionData{}, err
	}

	putAskIv, err := strconv.ParseFloat(record[26], 64)
	if err != nil {
		return eventmodels.OratsOptionData{}, err
	}

	residualRate, err := strconv.ParseFloat(record[27], 64)
	if err != nil {
		return eventmodels.OratsOptionData{}, err
	}

	delta, err := strconv.ParseFloat(record[28], 64)
	if err != nil {
		return eventmodels.OratsOptionData{}, err
	}

	gamma, err := strconv.ParseFloat(record[29], 64)
	if err != nil {
		return eventmodels.OratsOptionData{}, err
	}

	theta, err := strconv.ParseFloat(record[30], 64)
	if err != nil {
		return eventmodels.OratsOptionData{}, err
	}

	vega, err := strconv.ParseFloat(record[31], 64)
	if err != nil {
		return eventmodels.OratsOptionData{}, err
	}

	rho, err := strconv.ParseFloat(record[32], 64)
	if err != nil {
		return eventmodels.OratsOptionData{}, err
	}

	phi, err := strconv.ParseFloat(record[33], 64)
	if err != nil {
		return eventmodels.OratsOptionData{}, err
	}

	driftlessTheta, err := strconv.ParseFloat(record[34], 64)
	if err != nil {
		return eventmodels.OratsOptionData{}, err
	}

	callSmvVol, err := strconv.ParseFloat(record[35], 64)
	if err != nil {
		return eventmodels.OratsOptionData{}, err
	}

	putSmvVol, err := strconv.ParseFloat(record[36], 64)
	if err != nil {
		return eventmodels.OratsOptionData{}, err
	}

	extSmvVol, err := strconv.ParseFloat(record[37], 64)
	if err != nil {
		return eventmodels.OratsOptionData{}, err
	}

	extCallValue, err := strconv.ParseFloat(record[38], 64)
	if err != nil {
		return eventmodels.OratsOptionData{}, err
	}

	extPutValue, err := strconv.ParseFloat(record[39], 64)
	if err != nil {
		return eventmodels.OratsOptionData{}, err
	}

	spotPrice, err := strconv.ParseFloat(record[40], 64)
	if err != nil {
		return eventmodels.OratsOptionData{}, err
	}

	quoteDate, err := time.Parse(time.RFC3339, record[41])
	if err != nil {
		return eventmodels.OratsOptionData{}, err
	}

	updatedAt, err := time.Parse(time.RFC3339, record[42])
	if err != nil {
		return eventmodels.OratsOptionData{}, err
	}

	snapShotEstTime, err := time.Parse(time.RFC3339, record[43])
	if err != nil {
		return eventmodels.OratsOptionData{}, err
	}

	snapShotDate, err := time.Parse(time.RFC3339, record[44])
	if err != nil {
		return eventmodels.OratsOptionData{}, err
	}

	tickerId, err := strconv.Atoi(record[46])
	if err != nil {
		return eventmodels.OratsOptionData{}, err
	}

	monthId, err := strconv.Atoi(record[47])
	if err != nil {
		return eventmodels.OratsOptionData{}, err
	}

	return eventmodels.OratsOptionData{
		Ticker:           record[0],
		TradeDate:        record[1],
		ExpirDate:        record[2],
		Dte:              dte,
		Strike:           strike,
		StockPrice:       stockPrice,
		CallVolume:       callVolume,
		CallOpenInterest: callOpenInterest,
		CallBidSize:      callBidSize,
		CallAskSize:      callAskSize,
		PutVolume:        putVolume,
		PutOpenInterest:  putOpenInterest,
		PutBidSize:       putBidSize,
		PutAskSize:       putAskSize,
		CallBidPrice:     callBidPrice,
		CallValue:        callValue,
		CallAskPrice:     callAskPrice,
		PutBidPrice:      putBidPrice,
		PutValue:         putValue,
		PutAskPrice:      putAskPrice,
		CallBidIv:        callBidIv,
		CallMidIv:        callMidIv,
		CallAskIv:        callAskIv,
		SmvVol:           smvVol,
		PutBidIv:         putBidIv,
		PutMidIv:         putMidIv,
		PutAskIv:         putAskIv,
		ResidualRate:     residualRate,
		Delta:            delta,
		Gamma:            gamma,
		Theta:            theta,
		Vega:             vega,
		Rho:              rho,
		Phi:              phi,
		DriftlessTheta:   driftlessTheta,
		CallSmvVol:       callSmvVol,
		PutSmvVol:        putSmvVol,
		ExtSmvVol:        extSmvVol,
		ExtCallValue:     extCallValue,
		ExtPutValue:      extPutValue,
		SpotPrice:        spotPrice,
		QuoteDate:        quoteDate,
		UpdatedAt:        updatedAt,
		SnapShotEstTime:  snapShotEstTime,
		SnapShotDate:     snapShotDate,
		ExpiryTod:        record[45],
		TickerId:         tickerId,
		MonthId:          monthId,
	}, nil
}
