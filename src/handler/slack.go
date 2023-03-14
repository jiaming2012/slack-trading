package handler

import (
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"net/http"
	"slack-trading/src/models"
)

func SlackApiEventHandler(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)

	// currently we always return 200 back to the slack server as we do not have any advanced
	// error handling capabilities
	w.WriteHeader(200)

	var ev models.SlackEvent
	if err := decoder.Decode(&ev); err != nil {
		w.Write(nil)
		log.Errorf("Failed to decode slack event %v: %v", ev, err)
		return
	}

	switch msg := ev.GetType().(type) {
	case *models.SlackVerificationRequest:
		w.Write([]byte(msg.Challenge))
	case *models.IncomingSlackMessage:
		log.Infof("Incomming slack message: %v", msg)
		w.Write(nil)
	default:
		log.Errorf("Unknown slack message type: %v", msg)
	}
}
