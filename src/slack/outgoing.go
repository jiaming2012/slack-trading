package slack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"slack-trading/src/models"
	"time"
)

func SendResponse(msg string, url string, isEphemeral bool) {
	body := make(map[string]interface{})
	body["text"] = msg
	if isEphemeral {
		body["response_type"] = "ephemeral"
	} else {
		body["response_type"] = "in_channel"
	}

	go PostJSON(url, body)
}

func PostJSON(url string, body map[string]interface{}) ([]byte, error) {
	client := http.Client{
		Timeout: 60 * time.Second,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("PostJSON (Marshal): %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(payload))
	if err != nil {
		return nil, fmt.Errorf("PostJSON (NewRequest): %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	res, getErr := client.Do(req)
	if getErr != nil {
		return nil, fmt.Errorf("PostJSON (Do): %w", err)
	}

	if res.Body != nil {
		defer res.Body.Close()
	}

	bodyBytes, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		return nil, fmt.Errorf("PostJSON (ReadAll): %w", readErr)
	}

	if res.StatusCode >= 400 {
		var errDTO models.ErrorDTO
		if jsonErr := json.Unmarshal(bodyBytes, &errDTO); jsonErr != nil {
			return nil, fmt.Errorf("PostJSON (jsonErr): %w", jsonErr)
		}

		return nil, fmt.Errorf("errDTO.Msg: %v", errDTO.Msg)
	}

	return bodyBytes, nil
}
