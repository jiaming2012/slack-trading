package handler

import (
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"slack-trading/src/models"
)

func SlackApiEventHandler(w http.ResponseWriter, r *http.Request) {
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error(err)
	}

	log.Infof("payload: %s", payload)

	decoder := json.NewDecoder(r.Body)

	// currently we always return 200 back to the slack server as we do not have any advanced
	// error handling capabilities
	w.WriteHeader(200)

	var ev models.SlackEvent
	if err := decoder.Decode(&ev); err != nil {
		w.Write(nil)
		fmt.Printf("ERROR: decoding ev %v\n", ev)
		return
	}

	switch msg := ev.GetType().(type) {
	case *models.SlackVerificationRequest:
		log.Info("msg: ", msg)
		w.Write([]byte(msg.Challenge))
	case *models.IncomingSlackMessage:
		log.Info("msg: ", msg)
		w.Write(nil)

		fmt.Println("incoming message: ", msg)
	default:
		log.Info("msg default: ", msg)
	}
}
