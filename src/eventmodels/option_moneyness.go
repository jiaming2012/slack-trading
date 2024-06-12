package eventmodels

type OptionMoneyness string

const (
	OptionMoneynessIntheMoney    OptionMoneyness = "in_the_money"
	OptionMoneynessOutOfTheMoney OptionMoneyness = "out_of_the_money"
	OptionMoneynessAtTheMoney    OptionMoneyness = "at_the_money"
)
