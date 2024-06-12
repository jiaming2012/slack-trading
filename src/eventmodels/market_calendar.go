package eventmodels

type MarketCalendar struct {
	Calendar struct {
		Month int `json:"month"`
		Year  int `json:"year"`
		Days  struct {
			Day []struct {
				Date        string `json:"date"`
				Status      string `json:"status"`
				Description string `json:"description"`
				Premarket   struct {
					Start string `json:"start"`
					End   string `json:"end"`
				} `json:"premarket"`
				Open struct {
					Start string `json:"start"`
					End   string `json:"end"`
				} `json:"open"`
				Postmarket struct {
					Start string `json:"start"`
					End   string `json:"end"`
				} `json:"postmarket"`
			} `json:"day"`
		} `json:"days"`
	} `json:"calendar"`
}
