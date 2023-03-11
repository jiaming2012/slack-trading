package handler

import (
	"fmt"
	"net/url"
	"strings"
)

func validateFormAddRequest(data url.Values) (string, string, error) {
	paramsPayload, ok := data["text"]
	fmt.Println("data: ", data)
	fmt.Println(paramsPayload)
	if !ok {
		return "", "", fmt.Errorf("Could not find text\n")
	}

	if len(paramsPayload) != 1 {
		return "", "", fmt.Errorf("Invalid paramsPayload length: %d\n", len(paramsPayload))
	}

	params := strings.Fields(paramsPayload[0])
	if len(params) != 2 {
		return "", "", fmt.Errorf("Invalid params length: %d\n", len(params))
	}

	//if err := validateTokenAddress(params[1]); err != nil {
	//	return "", "", fmt.Errorf("Invalid token address %s. Error: %s\n", params[1], err)
	//}
	//
	//tokenName := params[0]
	//tokenAddress := params[1]

	return "", "", nil
}

func validateForm(data url.Values) (string, string, error) {
	// validate command
	cmd, ok := data["command"]
	if !ok {
		return "", "", fmt.Errorf("Could not find command\n")
	}

	if len(cmd) != 1 {
		return "", "", fmt.Errorf("Invalid cmd length: %d\n", len(cmd))
	}

	// validate response url
	responseURL, ok := data["response_url"]
	if !ok {
		return "", "", fmt.Errorf("Could not find response_url\n")
	}

	if len(responseURL) != 1 {
		return "", "", fmt.Errorf("Invalid response_url length: %d\n", len(responseURL))
	}

	return cmd[0], responseURL[0], nil
}
