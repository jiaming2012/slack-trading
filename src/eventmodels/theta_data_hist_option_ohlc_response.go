package eventmodels

import (
	"fmt"
)

type ThetaDataResponse struct {
	Header       ThetaDataResponseHeader `json:"header"`
	OhlcResponse [][]interface{}         `json:"response"`
}

func (r *ThetaDataResponse) StoreHeaderIndex(headerName string, headerIndexMap map[string]int) error {
	for i, v := range r.Header.Format {
		if v == headerName {
			headerIndexMap[v] = i
			return nil
		}
	}

	return fmt.Errorf("ThetaDataResponse: StoreHeaderIndex: unable to find header %v", headerName)
}

func (r *ThetaDataResponse) ToHistOptionOhlcDTO() ([]HistOptionOhlcDTO, error) {
	headers := make(map[string]int)

	if err := r.StoreHeaderIndex("ms_of_day", headers); err != nil {
		return nil, fmt.Errorf("ThetaDataHistOptionOHLCResponse: %w", err)
	}

	if err := r.StoreHeaderIndex("open", headers); err != nil {
		return nil, fmt.Errorf("ThetaDataHistOptionOHLCResponse: %w", err)
	}

	if err := r.StoreHeaderIndex("high", headers); err != nil {
		return nil, fmt.Errorf("ThetaDataHistOptionOHLCResponse: %w", err)
	}

	if err := r.StoreHeaderIndex("low", headers); err != nil {
		return nil, fmt.Errorf("ThetaDataHistOptionOHLCResponse: %w", err)
	}

	if err := r.StoreHeaderIndex("close", headers); err != nil {
		return nil, fmt.Errorf("ThetaDataHistOptionOHLCResponse: %w", err)
	}

	if err := r.StoreHeaderIndex("volume", headers); err != nil {
		return nil, fmt.Errorf("ThetaDataHistOptionOHLCResponse: %w", err)
	}

	if err := r.StoreHeaderIndex("date", headers); err != nil {
		return nil, fmt.Errorf("ThetaDataHistOptionOHLCResponse: %w", err)
	}

	ticks, err := getTicks(headers, r.OhlcResponse)
	if err != nil {
		return nil, fmt.Errorf("ThetaDataHistOptionOHLCResponse: failed to convert ticks: %w", err)
	}

	return ticks, nil
}

func getTicks(headers map[string]int, ticks [][]interface{}) ([]HistOptionOhlcDTO, error) {
	out := make([]HistOptionOhlcDTO, 0)

	for _, row := range ticks {
		msOfDay, ok := row[headers["msOfDay"]].(float64)
		if !ok {
			return nil, fmt.Errorf("getTicks: msOfDay, unable to convert ms_of_day to float64, %v", row[0])
		}
		open, ok := row[headers["open"]].(float64)
		if !ok {
			return nil, fmt.Errorf("getTicks: open, unable to convert open to float64, %v", row[2])
		}
		high, ok := row[headers["high"]].(float64)
		if !ok {
			return nil, fmt.Errorf("getTicks: high, unable to convert high to float64, %v", row[3])
		}
		low, ok := row[headers["low"]].(float64)
		if !ok {
			return nil, fmt.Errorf("getTicks: low, unable to convert low to float64, %v", row[4])
		}
		close, ok := row[headers["close"]].(float64)
		if !ok {
			return nil, fmt.Errorf("getTicks: close, unable to convert close to float64, %v", row[5])
		}
		volume, ok := row[headers["volume"]].(float64)
		if !ok {
			return nil, fmt.Errorf("getTicks: volume, unable to convert volume to float64, %v", row[6])
		}
		date, ok := row[headers["date"]].(float64)
		if !ok {
			return nil, fmt.Errorf("getTicks: date, unable to convert date to string, %v", row[7])
		}

		if volume <= 0 {
			continue
		}

		out = append(out, HistOptionOhlcDTO{
			MsOfDay: int(msOfDay),
			Open:    open,
			High:    high,
			Low:     low,
			Close:   close,
			Volume:  int(volume),
			Date:    int(date),
		})
	}

	return out, nil
}
