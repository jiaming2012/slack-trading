package eventmodels

type OptionChain struct {
	Calls          []OptionContractID `json:"calls"`
	Puts           []OptionContractID `json:"puts"`
	ExpirationDate string             `json:"expiration_date"`
	ExpirationType string             `json:"expiration_type"`
	Underlying     string             `json:"underlying"`
}
