package eventmodels

import (
	"fmt"
	"log"
	"strconv"
	"time"
)

type TradingViewCandleDTO struct {
	SavedEventParms SavedEventParameters `json:"-"`
	Timestamp       string               `csv:"time" json:"time"`
	Open            string               `csv:"open" json:"open"`
	High            string               `csv:"high" json:"high"`
	Low             string               `csv:"low" json:"low"`
	Close           string               `csv:"close" json:"close"`
	UpTrend         string               `csv:"Up Trend" json:"UpTrend"`
	UpTrendBegins   string               `csv:"UpTrend Begins" json:"UpTrend Begins"`
	DownTrend       string               `csv:"Down Trend" json:"Down Trend"`
	DownTrendBegins string               `csv:"DownTrend Begins" json:"DownTrend Begins"`
	K               string               `csv:"K" json:"K"`
	D               string               `csv:"D" json:"D"`
}

func (dto *TradingViewCandleDTO) GetSavedEventParameters() SavedEventParameters {
	return dto.SavedEventParms
}

func (dto *TradingViewCandleDTO) GetMetaData() *MetaData {
	return &MetaData{}
}

func NewCsvCandleDTO(streamName StreamName, eventName EventName, schemaVersion int) *TradingViewCandleDTO {
	return &TradingViewCandleDTO{
		SavedEventParms: SavedEventParameters{
			StreamName:    streamName,
			EventName:     eventName,
			SchemaVersion: schemaVersion,
		},
	}
}

func (dto *TradingViewCandleDTO) ToModel() *TradingViewCandle {
	t, err := time.Parse(time.RFC3339, dto.Timestamp)
	if err != nil {
		t, err = time.Parse("2006-01-02", dto.Timestamp)
		if err != nil {
			log.Fatal(fmt.Errorf("error parsing time: %v", err))
		}
	}

	open, err := strconv.ParseFloat(dto.Open, 64)
	if err != nil {
		log.Fatal(fmt.Errorf("error parsing Open: %v", err))
	}

	high, err := strconv.ParseFloat(dto.High, 64)
	if err != nil {
		log.Fatal(fmt.Errorf("error parsing High: %v", err))
	}

	low, err := strconv.ParseFloat(dto.Low, 64)
	if err != nil {
		log.Fatal(fmt.Errorf("error parsing Low: %v", err))
	}

	close, err := strconv.ParseFloat(dto.Close, 64)
	if err != nil {
		log.Fatal(fmt.Errorf("error parsing Close: %v", err))
	}

	var upTrend float64
	if dto.UpTrend == "NaN" || dto.UpTrend == "" {
		upTrend = 0
	} else {
		upTrend, err = strconv.ParseFloat(dto.UpTrend, 64)
		if err != nil {
			log.Fatal(fmt.Errorf("error parsing UpTrend: %v", err))
		}
	}

	var downTrend float64
	if dto.DownTrend == "NaN" || dto.DownTrend == "" {
		downTrend = 0
	} else {
		downTrend, err = strconv.ParseFloat(dto.DownTrend, 64)
		if err != nil {
			log.Fatal(fmt.Errorf("error parsing DownTrend: %v", err))
		}
	}

	var upTrendBegins float64
	if dto.UpTrendBegins == "NaN" || dto.UpTrendBegins == "" {
		upTrendBegins = 0
	} else {
		upTrendBegins, err = strconv.ParseFloat(dto.UpTrendBegins, 64)
		if err != nil {
			log.Fatal(fmt.Errorf("error parsing UpTrendBegins: %v", err))
		}
	}

	var downTrendBegins float64
	if dto.DownTrendBegins == "NaN" || dto.DownTrendBegins == "" {
		downTrendBegins = 0
	} else {
		downTrendBegins, err = strconv.ParseFloat(dto.DownTrendBegins, 64)
		if err != nil {
			log.Fatal(fmt.Errorf("error parsing DownTrendBegins: %v", err))
		}
	}

	var k float64
	if dto.K == "NaN" || dto.K == "" {
		k = 0
	} else {
		k, err = strconv.ParseFloat(dto.K, 64)
		if err != nil {
			log.Fatal(fmt.Errorf("error parsing K: %v", err))
		}
	}

	var d float64
	if dto.D == "NaN" || dto.D == "" {
		d = 0
	} else {
		d, err = strconv.ParseFloat(dto.D, 64)
		if err != nil {
			log.Fatal(fmt.Errorf("error parsing D: %v", err))
		}
	}

	return &TradingViewCandle{
		Open:            open,
		High:            high,
		Low:             low,
		Close:           close,
		UpTrend:         upTrend,
		DownTrend:       downTrend,
		UpTrendBegins:   upTrendBegins,
		DownTrendBegins: downTrendBegins,
		K:               k,
		D:               d,
		Timestamp:       t,
	}
}
