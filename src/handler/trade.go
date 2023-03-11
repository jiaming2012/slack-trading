package handler

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"net/http"
	"slack-trading/src/slack"
)

func Trade(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		r.ParseForm()

		cmd, responseURL, err := validateForm(r.Form)
		if err != nil {
			log.Error(err)
			return
		}

		if cmd == "/add" {
			_, _, validationErr := validateFormAddRequest(r.Form)
			if validationErr != nil {
				log.Error(validationErr)
				return
			}

			//if err != nil {
			//	go slackSendResponse(fmt.Sprintf("Failed to add token due to %v", err), responseURL, true)
			//	return
			//}
			//
			//token := &models.TradableToken{
			//	Name:    tokenName,
			//	Address: tokenAddress,
			//	ChainId: 56,
			//}
			msg := fmt.Sprintf("cool")
			//go defitrader.InsertToken(token)
			go slack.SendResponse(msg, responseURL, true)
		}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprintf(w, "traderHandler: unsupported method")
	}
}