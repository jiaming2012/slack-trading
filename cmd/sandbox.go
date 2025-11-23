package main

import (
	"fmt"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func main() {
	o1, err := eventmodels.NewOptionSymbolFromString("O:AAPL250905C00230000")
	if err != nil {
		panic(err)
	}

	o2, err := eventmodels.NewOptionSymbolFromString("O:AAPL250905C00230000")
	if err != nil {
		panic(err)
	}

	s1 := eventmodels.OptionContractV3{
		Symbol: o1,
	}

	s2 := eventmodels.OptionContractV3{
		Symbol: o2,
	}

	m := make(map[string]int)
	m[s1.GetTicker()] = 1
	m[s2.GetTicker()] = 2

	for k, v := range m {
		fmt.Printf("Key: %+v, Value: %d\n", k, v)
	}
}
