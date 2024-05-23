package eventmodels

type OptionLadder struct {
	AtTheMoneyStrike float64
	CallsAboveStrike []OptionContractV1
	CallsBelowStrike []OptionContractV1
	PutsAboveStrike  []OptionContractV1
	PutsBelowStrike  []OptionContractV1
}
