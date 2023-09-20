package signalapi

import (
	"github.com/gorilla/mux"
	"net/http"
	"slack-trading/src/eventmodels"
	"slack-trading/src/eventproducers"
	pubsub "slack-trading/src/eventpubsub"
)

/*
{
  "strategies": {
    "name": "",
    "conditions": [{
	  "entry": {
		 "name": "",
		 "isSatisfied" true
	   },
	  "exit": {
		 "name": "",
		 "isSatisfied" true
	  }
	}]
  }
}
*/

//type SignalsResultItem struct {
//	EntryConditions []*models.Condition `json:"conditions"`
//}

//type NewSignalsResult struct {
//	RequestID  uuid.UUID               `json:"requestID"`
//	Strategies []*SignalsResultItem `json:"strategies"`
//}
//
//func (r *NewSignalsResult) GetRequestID() uuid.UUID {
//	return r.RequestID
//}

func signalsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		eventproducers.SignalRequestHandler(pubsub.NewSignalsRequest, &eventmodels.SignalRequest{}, &eventmodels.NewSignalResult{}, w, r)
	} else {
		w.WriteHeader(404)
	}
}

func SetupHandler(router *mux.Router) {
	router.HandleFunc("", signalsHandler)
}
