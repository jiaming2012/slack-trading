package coingecko

import (
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"slack-trading/src/models"
	"slack-trading/src/utils"
)

func FetchPrice(symbol string) (*models.GeckoCoin, error) {
	switch symbol {
	case "bitcoin":
	default:
		return nil, fmt.Errorf("FetchPrice: %s is not configured", symbol)
	}

	url := "https://api.coingecko.com/api/v3/coins/bitcoin?developer_data=false&community_data=false&market_data=false&localization=false"

	bytes, err := utils.Get(url)
	if err != nil {
		return nil, err
	}

	var coin models.GeckoCoin
	jsonErr := json.Unmarshal(bytes, &coin)
	if jsonErr != nil {
		log.Fatal(jsonErr)
	}

	return &coin, nil
}
