package utils

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"slack-trading/src/models"
	"time"
)

func Get(url string) ([]byte, error) {
	client := http.Client{
		Timeout: 120 * time.Second, // give time for exponential backoff for calls to web3
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("client Get: %w", err)
	}

	res, getErr := client.Do(req)
	if getErr != nil {
		return nil, fmt.Errorf("getErr Get: %w", getErr)
	}

	if res.Body != nil {
		defer res.Body.Close()
	}

	body, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		return nil, fmt.Errorf("readErr Get: %w", readErr)
	}

	if res.StatusCode >= 400 {
		var errDTO models.ErrorDTO
		if jsonErr := json.Unmarshal(body, &errDTO); jsonErr != nil {
			return nil, fmt.Errorf("jsonErr Get: %w", jsonErr)
		}

		return nil, fmt.Errorf("errDTO Get: %v", errDTO.Msg)
	}

	return body, nil
}
