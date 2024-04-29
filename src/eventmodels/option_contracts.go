package eventmodels

type OptionContracts []*OptionContract

func (c OptionContracts) GetListOfExpirations() []string {
	expirationsMap := make(map[string]struct{})
	for _, contract := range c {
		expirationsMap[contract.Expiration.Format("2006-01-02")] = struct{}{}
	}

	expirations := make([]string, 0, len(expirationsMap))
	for expiration := range expirationsMap {
		expirations = append(expirations, expiration)
	}

	return expirations
}
