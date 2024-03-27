package eventproducers

import (
	"net/http"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"slack-trading/src/eventmodels"
	pubsub "slack-trading/src/eventpubsub"
	"slack-trading/src/models"
)

type ApiRequest2 interface {
	ParseHTTPRequest(r *http.Request) error
	Validate(r *http.Request) error
	GetMetaData() *eventmodels.MetaData
	SetMetaData(*eventmodels.MetaData)
}

type ApiRequest interface {
	ParseHTTPRequest(r *http.Request) error
	Validate(r *http.Request) error
	SetRequestID(id uuid.UUID)
	GetMetaData() *eventmodels.MetaData
	GetRequestID() uuid.UUID
}

type SignalRequest interface {
	ApiRequest
	GetSource() models.RequestSource
}

func ApiRequestHandler2(eventName pubsub.EventName, req ApiRequest2, resp any, w http.ResponseWriter, r *http.Request) {
	if err := req.ParseHTTPRequest(r); err != nil {
		if respErr := SetErrorResponse("parser", 400, err, w); respErr != nil {
			log.Errorf("ApiRequestHandler: failed to parse http parameters: %v", respErr)
		}
		return
	}

	if err := req.Validate(r); err != nil {
		if respErr := SetErrorResponse("validation", 400, err, w); respErr != nil {
			log.Errorf("ApiRequestHandler: failed to validate http request: %v", respErr)
		}
		return
	}

	// todo: idea? save the request to eventstore db???
	// document adding a new request endpoint

	id := uuid.New()

	req.SetMetaData(&eventmodels.MetaData{
		RequestID: id,
	})

	resultCh, errCh := eventmodels.RegisterResultCallback(id)

	pubsub.PublishResult2("GenericHandler", eventName, req)

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

func ApiRequestHandler[Request ApiRequest, Response any](eventName pubsub.EventName, req Request, resp Response, w http.ResponseWriter, r *http.Request) {
	if err := req.ParseHTTPRequest(r); err != nil {
		if respErr := SetErrorResponse("parser", 400, err, w); respErr != nil {
			log.Errorf("ApiRequestHandler: failed to parse http parameters: %v", respErr)
		}
		return
	}

	if err := req.Validate(r); err != nil {
		if respErr := SetErrorResponse("validation", 400, err, w); respErr != nil {
			log.Errorf("ApiRequestHandler: failed to validate http request: %v", respErr)
		}
		return
	}

	// todo: idea? save the request to eventstore db???
	// document adding a new request endpoint

	id := uuid.New()

	req.SetRequestID(id)

	resultCh, errCh := eventmodels.RegisterResultCallback(id)

	pubsub.PublishResult("GenericHandler", eventName, req)

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
