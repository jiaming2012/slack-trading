package handler

import (
	"fmt"
	"net/url"
)

func validateForm(data url.Values) (string, string, error) {
	// validate command
	cmd, ok := data["command"]
	if !ok {
		return "", "", fmt.Errorf("could not find command")
	}

	if len(cmd) != 1 {
		return "", "", fmt.Errorf("invalid cmd length: %d", len(cmd))
	}

	// validate response url
	responseURL, ok := data["response_url"]
	if !ok {
		return "", "", fmt.Errorf("could not find response_url")
	}

	if len(responseURL) != 1 {
		return "", "", fmt.Errorf("invalid response_url length: %d\n", len(responseURL))
	}

	return cmd[0], responseURL[0], nil
}
