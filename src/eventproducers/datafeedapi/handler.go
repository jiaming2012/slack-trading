package datafeedapi

import (
	"fmt"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"net/http"
	"slack-trading/src/eventmodels"
	"slack-trading/src/eventproducers"
	pubsub "slack-trading/src/eventpubsub"
)

func datafeedHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		vars := mux.Vars(r)
		if feedName, found := vars["feedName"]; found {
			switch feedName {
			case "manual":
				eventproducers.ApiRequestHandler(pubsub.ManualDatafeedUpdateRequest, &eventmodels.ManualDatafeedUpdateRequest{}, &eventmodels.ManualDatafeedUpdateResult{}, w, r)
			default:
				err := fmt.Errorf("unknown feedName, found %v", feedName)
				if respErr := eventproducers.SetErrorResponse("request", 400, err, w); respErr != nil {
					log.Errorf("datafeedHandler: invalid feed name - failed to set error response: %v", respErr)
					w.WriteHeader(500)
				}
			}

			return
		}

		err := fmt.Errorf("feedName not found in url parameters")
		if respErr := eventproducers.SetErrorResponse("request", 400, err, w); respErr != nil {
			log.Errorf("datafeedHandler: failed to set error response: %v", respErr)
			w.WriteHeader(500)
			return
		}
	} else {
		w.WriteHeader(404)
	}
}

func SetupHandler(router *mux.Router) {
	router.HandleFunc("/{feedName}", datafeedHandler)
}
