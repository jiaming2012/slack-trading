package eventmodels

import "time"

type AggregateBarWithIndicators struct {
	Timestamp            time.Time `json:"datetime"`
	Open                 float64   `json:"open"`
	Close                float64   `json:"close"`
	High                 float64   `json:"high"`
	Low                  float64   `json:"low"`
	Volume               float64   `json:"volume"`
	SuperT_50_3          float64   `json:"SUPERT_50_3.0"`
	SuperD_50_3          int       `json:"SUPERTd_50_3.0"`
	SuperL_50_3          float64   `json:"SUPERTl_50_3.0"`
	SuperS_50_3          float64   `json:"SUPERTs_50_3.0"`
	StochRsiK_14_14_3_3  float64   `json:"STOCHRSIk_14_14_3_3"`
	StochRsiD_14_14_3_3  float64   `json:"STOCHRSId_14_14_3_3"`
	ATRr_14              float64   `json:"ATRr_14"`
	Sma50                float64   `json:"SMA_50"`
	Sma100               float64   `json:"SMA_100"`
	Sma200               float64   `json:"SMA_200"`
	StochRsiCrossAbove20 bool      `json:"stochrsi_cross_above_20"`
	StochRsiCrossBelow80 bool      `json:"stochrsi_cross_below_80"`
	CloseLag1            float64   `json:"close_lag_1"`
	CloseLag2            float64   `json:"close_lag_2"`
	CloseLag3            float64   `json:"close_lag_3"`
	CloseLag4            float64   `json:"close_lag_4"`
	CloseLag5            float64   `json:"close_lag_5"`
	CloseLag6            float64   `json:"close_lag_6"`
	CloseLag7            float64   `json:"close_lag_7"`
	CloseLag8            float64   `json:"close_lag_8"`
	CloseLag9            float64   `json:"close_lag_9"`
	CloseLag10           float64   `json:"close_lag_10"`
	CloseLag11           float64   `json:"close_lag_11"`
	CloseLag12           float64   `json:"close_lag_12"`
	CloseLag13           float64   `json:"close_lag_13"`
	CloseLag14           float64   `json:"close_lag_14"`
	CloseLag15           float64   `json:"close_lag_15"`
	CloseLag16           float64   `json:"close_lag_16"`
	CloseLag17           float64   `json:"close_lag_17"`
	CloseLag18           float64   `json:"close_lag_18"`
	CloseLag19           float64   `json:"close_lag_19"`
	CloseLag20           float64   `json:"close_lag_20"`
}

func (a *AggregateBarWithIndicators) ToPolygonAggregateBarV2() *PolygonAggregateBarV2 {
	return &PolygonAggregateBarV2{
		Volume:    a.Volume,
		Open:      a.Open,
		Close:     a.Close,
		High:      a.High,
		Low:       a.Low,
		Timestamp: a.Timestamp,
	}
}
