package eventmodels

type OptionChain struct {
	Calls          []EventStreamID `json:"calls"`
	Puts           []EventStreamID `json:"puts"`
	ExpirationDate string          `json:"expiration_date"`
	ExpirationType string          `json:"expiration_type"`
	Underlying     string          `json:"underlying"`
}
