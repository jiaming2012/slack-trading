package models

type OptionChainDTO struct {
	Values []*OptionChainTickDTO `json:"option"`
}
