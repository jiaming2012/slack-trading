package eventmodels

type CandleData struct {
	Date  []string  `json:"Date"`
	Open  []float64 `json:"Open"`
	High  []float64 `json:"High"`
	Low   []float64 `json:"Low"`
	Close []float64 `json:"Close"`
}

type OrderData struct {
	Date         []string  `json:"Date"`
	Type         []string  `json:"Type"`
	Price        []float64 `json:"Price"`
	StrikePriceA float64   `json:"StrikePriceA"`
	StrikePriceB float64   `json:"StrikePriceB"`
}

type ChartData struct {
	Title          string `json:"title"`
	Sublplot1Title string `json:"subplot_1_title"`
	Sublplot2Title string `json:"subplot_2_title"`
}

type PlotOrderInputData struct {
	ChartData  ChartData  `json:"chart_data"`
	CandleData CandleData `json:"candle_data"`
	OrderData  OrderData  `json:"order_data"`
	OptionData CandleData `json:"option_data"`
}
