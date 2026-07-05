package datafeedapi

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/go/models"
	"github.com/jiaming2012/slack-trading/src/go/api"
)

func datafeedHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		vars := mux.Vars(r)
		if feedName, found := vars["feedName"]; found {
			switch feedName {
			case "manual":
				api.ApiRequestHandler2(models.ManualDatafeedUpdateRequestEventName, &models.ManualDatafeedUpdateRequest{}, &models.ManualDatafeedUpdateResult{}, w, r)
			default:
				err := fmt.Errorf("unknown feedName, found %v", feedName)
				if respErr := api.SetErrorResponse("request", 400, err, w); respErr != nil {
					log.Errorf("datafeedHandler: invalid feed name - failed to set error response: %v", respErr)
					w.WriteHeader(500)
				}
			}

			return
		}

		err := fmt.Errorf("feedName not found in url parameters")
		if respErr := api.SetErrorResponse("request", 400, err, w); respErr != nil {
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
