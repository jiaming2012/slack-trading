package eventmodels

type OptionLadderV3 struct {
	AtTheMoneyStrike float64
	CallsAboveStrike []OptionContractV3
	CallsBelowStrike []OptionContractV3
	PutsAboveStrike  []OptionContractV3
	PutsBelowStrike  []OptionContractV3
}

type OptionLadder struct {
	AtTheMoneyStrike float64
	CallsAboveStrike []OptionContractV1
	CallsBelowStrike []OptionContractV1
	PutsAboveStrike  []OptionContractV1
	PutsBelowStrike  []OptionContractV1
}
