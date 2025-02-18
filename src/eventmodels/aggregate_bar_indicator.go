package eventmodels

import (
	"time"

	pb "github.com/jiaming2012/slack-trading/src/playground"
)

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
	Hammer               float64   `json:"CDL_HAMMER"`
	Doji                 float64   `json:"CDL_DOJI_10_0.1"`
}

func (a *AggregateBarWithIndicators) ToProto() *pb.Bar {
	return &pb.Bar{
		Open:                  a.Open,
		High:                  a.High,
		Low:                   a.Low,
		Close:                 a.Close,
		Volume:                a.Volume,
		Datetime:              a.Timestamp.UTC().Format("2006-01-02T15:04:05Z"),
		SuperT_50_3:           a.SuperT_50_3,
		SuperD_50_3:           int32(a.SuperD_50_3),
		SuperL_50_3:           a.SuperL_50_3,
		SuperS_50_3:           a.SuperS_50_3,
		StochrsiK_14_14_3_3:   a.StochRsiK_14_14_3_3,
		StochrsiD_14_14_3_3:   a.StochRsiD_14_14_3_3,
		Atr_14:                a.ATRr_14,
		Sma_50:                a.Sma50,
		Sma_100:               a.Sma100,
		Sma_200:               a.Sma200,
		StochrsiCrossAbove_20: a.StochRsiCrossAbove20,
		StochrsiCrossBelow_80: a.StochRsiCrossBelow80,
		CloseLag_1:            a.CloseLag1,
		CloseLag_2:            a.CloseLag2,
		CloseLag_3:            a.CloseLag3,
		CloseLag_4:            a.CloseLag4,
		CloseLag_5:            a.CloseLag5,
		CloseLag_6:            a.CloseLag6,
		CloseLag_7:            a.CloseLag7,
		CloseLag_8:            a.CloseLag8,
		CloseLag_9:            a.CloseLag9,
		CloseLag_10:           a.CloseLag10,
		CloseLag_11:           a.CloseLag11,
		CloseLag_12:           a.CloseLag12,
		CloseLag_13:           a.CloseLag13,
		CloseLag_14:           a.CloseLag14,
		CloseLag_15:           a.CloseLag15,
		CloseLag_16:           a.CloseLag16,
		CloseLag_17:           a.CloseLag17,
		CloseLag_18:           a.CloseLag18,
		CloseLag_19:           a.CloseLag19,
		CloseLag_20:           a.CloseLag20,
		CdlHammer:             a.Hammer,
		CdlDoji_10_0_1:        a.Doji,
	}
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
