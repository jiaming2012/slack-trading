package eventservices

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func FetchOratsDataMock(ticker, token string, fromDate, toDate time.Time) ([]eventmodels.OratsOptionData, error) {
	csvData := `ticker,tradeDate,expirDate,dte,strike,stockPrice,callVolume,callOpenInterest,callBidSize,callAskSize,putVolume,putOpenInterest,putBidSize,putAskSize,callBidPrice,callValue,callAskPrice,putBidPrice,putValue,putAskPrice,callBidIv,callMidIv,callAskIv,smvVol,putBidIv,putMidIv,putAskIv,residualRate,delta,gamma,theta,vega,rho,phi,driftlessTheta,callSmvVol,putSmvVol,extSmvVol,extCallValue,extPutValue,spotPrice,quoteDate,updatedAt,snapShotEstTime,snapShotDate,expiryTod,tickerId,monthId
	AAPL,2022-06-08,2022-09-16,101,160,149.67,150,28206,253,510,7,18438,124,104,5.8,5.860204002795059,5.9,15.55,15.641054396787531,15.7,0.3063158772183223,0.30801686256479954,0.3097178479112768,0.308,0.3038649107227709,0.3064286008767878,0.3089922910308047,-0.008209120374803128,0.38161255239927655,0.015355889799308844,-0.047622767388992515,0.2939141986686304,0.13545210863618393,-0.15118639086979158,-0.0459052133276907,0.3083639997493957,0.3069773808630928,0.3215480409289392,6.247634998832532,16.067895502105014,149.67,2022-06-08T13:59:50Z,2022-06-08T13:59:51Z,2022-06-08T14:00:01Z,2022-06-08T14:00:01Z,pm,101594,9
	AAPL,2022-06-08,2022-09-16,101,160,149.45,158,28206,1,215,7,18438,229,102,5.75,5.78583812885739,5.8,15.7,15.782968253643883,15.85,0.30745849466065145,0.3083088916342891,0.3091592886079268,0.308,0.30432705746682837,0.30689074762084534,0.3094544377748623,-0.008209120374803128,0.37823425664342863,0.015355889799308844,-0.047622767388992515,0.2939141986686304,0.13545210863618393,-0.15118639086979158,-0.0459052133276907,0.3086775601134979,0.3071631227329991,0.3215480409289392,6.1640518498378345,16.204312353110314,149.45,2022-06-08T14:00:55Z,2022-06-08T14:00:56Z,2022-06-08T14:01:01Z,2022-06-08T14:01:01Z,pm,101594,9
	AAPL,2022-06-08,2022-09-16,101,160,149.12,159,28206,152,1299,13,18438,234,132,5.6,5.66120101953646,5.7,15.9,15.991901944264272,16.05,0.3065732423106423,0.30827422765711954,0.3099752130035968,0.308,0.3041222758150914,0.30668596596910835,0.3092496561231253,-0.008209120374803128,0.37316681300965693,0.015355889799308844,-0.047622767388992515,0.2939141986686304,0.13545210863618393,-0.15118639086979158,-0.0459052133276907,0.30865528305906204,0.3072637172770291,0.3215480409289392,6.040070673345081,16.410331176617547,149.12,2022-06-08T14:02:00Z,2022-06-08T14:02:01Z,2022-06-08T14:02:02Z,2022-06-08T14:02:02Z,pm,101594,9`

	options, err := ParseCSV(csvData)
	if err != nil {
		return nil, fmt.Errorf("error parsing CSV: %v", err)
	}

	return options, nil
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

// ParseCSV parses CSV data into a slice of OratsOptionData.
func ParseCSV(data string) ([]eventmodels.OratsOptionData, error) {
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
		return eventmodels.OratsOptionData{}, err
	}

	strike, err := strconv.ParseFloat(record[4], 64)
	if err != nil {
		return eventmodels.OratsOptionData{}, err
	}

	stockPrice, err := strconv.ParseFloat(record[5], 64)
	if err != nil {
		return eventmodels.OratsOptionData{}, err
	}

	callVolume, err := strconv.Atoi(record[6])
	if err != nil {
		return eventmodels.OratsOptionData{}, err
	}

	callOpenInterest, err := strconv.Atoi(record[7])
	if err != nil {
		return eventmodels.OratsOptionData{}, err
	}

	callBidSize, err := strconv.Atoi(record[8])
	if err != nil {
		return eventmodels.OratsOptionData{}, err
	}

	callAskSize, err := strconv.Atoi(record[9])
	if err != nil {
		return eventmodels.OratsOptionData{}, err
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
