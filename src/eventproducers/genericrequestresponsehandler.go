package eventproducers

import (
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"net/http"
	"slack-trading/src/eventmodels"
	pubsub "slack-trading/src/eventpubsub"
)

type ApiRequest interface {
	ParseHTTPRequest(r *http.Request) error
	SetRequestID(id uuid.UUID)
}

type SignalRequest interface {
	ApiRequest
	GetSource() eventmodels.RequestSource
}

func SignalRequestHandler[Request SignalRequest, Response any](eventName pubsub.EventName, req Request, resp Response, w http.ResponseWriter, r *http.Request) {
	if err := req.ParseHTTPRequest(r); err != nil {
		if respErr := SetErrorResponse("validation", 400, err, w); respErr != nil {
			log.Errorf("GenericHandler: failed to set error response: %v", respErr)
		}
		return
	}

	id := uuid.New()
	req.SetRequestID(id)

	if req.GetSource() == eventmodels.WebClient {
		resultCh, errCh := eventmodels.RegisterResultCallback(id)

		pubsub.Publish("GenericHandler", eventName, req)

		select {
		case result := <-resultCh:
			if err := SetGenericResponse(result, w); err != nil {
				log.Errorf("GenericHandler: failed to set response: %v", err)
				w.WriteHeader(500)
				return
			}
		case err := <-errCh:
			if respErr := SetErrorResponse("req", 400, err, w); respErr != nil {
				log.Errorf("GenericHandler: failed to set error response: %v", respErr)
				w.WriteHeader(500)
				return
			}
		}
	} else {
		w.WriteHeader(200)
		pubsub.Publish("GenericHandler", eventName, req)
	}
}

func ApiRequestHandler[Request ApiRequest, Response any](eventName pubsub.EventName, req Request, resp Response, w http.ResponseWriter, r *http.Request) {
	if err := req.ParseHTTPRequest(r); err != nil {
		if respErr := SetErrorResponse("validation", 400, err, w); respErr != nil {
			log.Errorf("GenericHandler: failed to set error response: %v", respErr)
		}
		return
	}

	id := uuid.New()
	req.SetRequestID(id)
	resultCh, errCh := eventmodels.RegisterResultCallback(id)

	pubsub.Publish("GenericHandler", eventName, req)

	select {
	case result := <-resultCh:
		if err := SetGenericResponse(result, w); err != nil {
			log.Errorf("GenericHandler: failed to set response: %v", err)
			w.WriteHeader(500)
			return
		}
	case err := <-errCh:
		if respErr := SetErrorResponse("req", 400, err, w); respErr != nil {
			log.Errorf("GenericHandler: failed to set error response: %v", respErr)
			w.WriteHeader(500)
			return
		}
	}
}
