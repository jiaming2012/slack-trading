package handler

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/schema"
	log "github.com/sirupsen/logrus"

	"slack-trading/src/eventmodels"
	"slack-trading/src/eventpubsub"
	models "slack-trading/src/models"
)

func SlackApiEventHandler(w http.ResponseWriter, r *http.Request) {
	// currently we always return 200 back to the slack server as we do not have any advanced
	// error handling capabilities
	w.WriteHeader(200)

	contentType := r.Header.Get("Content-Type")
	switch contentType {
	case "application/x-www-form-urlencoded":
		if err := r.ParseForm(); err != nil {
			log.Errorf("SlackApiEventHandler: failed to parse form: %v", err)
			return
		}

		req := new(eventmodels.IncomingSlackRequest)
		schema.NewDecoder().Decode(req, r.Form)
		eventpubsub.PublishEventResult("SlackApiEventHandler", eventpubsub.GetAccountsRequestEvent, *req)
	case "application/json":
		decoder := json.NewDecoder(r.Body)

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
	default:
		log.Errorf("SlackApiEventHandler: unknown Content-Type: %v", contentType)
	}
}
