package eventproducers

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
	pubsub "github.com/jiaming2012/slack-trading/src/eventpubsub"
	"github.com/jiaming2012/slack-trading/src/models"
)

type SignalRequest interface {
	ApiRequest2
	GetSource() models.RequestSource
}

func ApiRequestHandler3(ctx context.Context, req eventmodels.ApiRequest3, requestExector eventmodels.RequestExecutor, w http.ResponseWriter, r *http.Request) {
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

	// todo: like the idea of automatically assinging a request id
	// id := uuid.New()

	// meta := &eventmodels.MetaData{
	// 	RequestID:         id,
	// 	IsExternalRequest: true,
	// }

	resultCh := make(chan interface{})
	errCh := make(chan error)
	
	go requestExector.Serve(r, req, resultCh, errCh)

	select {
	case result := <-resultCh:
		if err := SetGenericResponse(result, w); err != nil {
			log.Errorf("ApiRequestHandler3: failed to set response: %v", err)
			w.WriteHeader(500)
			return
		}
	case err := <-errCh:
		if respErr := SetErrorResponse("req", 400, err, w); respErr != nil {
			log.Errorf("ApiRequestHandler3: failed to set error response: %v", respErr)
			w.WriteHeader(500)
			return
		}
	}
}

func ApiRequestHandler2(eventName eventmodels.EventName, req ApiRequest2, resp any, w http.ResponseWriter, r *http.Request) {
	if err := req.ParseHTTPRequest(r); err != nil {
		if respErr := SetErrorResponse("parser", 400, err, w); respErr != nil {
			log.Errorf("ApiRequestHandler2: failed to parse http parameters: %v", respErr)
		}
		return
	}

	if err := req.Validate(r); err != nil {
		if respErr := SetErrorResponse("validation", 400, err, w); respErr != nil {
			log.Errorf("ApiRequestHandler2: failed to validate http request: %v", respErr)
		}
		return
	}

	// todo: idea? save the request to eventstore db???
	// document adding a new request endpoint

	id := uuid.New()

	meta := &eventmodels.MetaData{
		RequestID:         id,
		IsExternalRequest: true,
	}

	resultCh, errCh := eventmodels.RegisterResultCallback(id)

	pubsub.PublishResponse("ApiRequestHandler2", eventName, req, meta)

	select {
	case result := <-resultCh:
		if err := SetGenericResponse(result, w); err != nil {
			log.Errorf("ApiRequestHandler2: failed to set response: %v", err)
			w.WriteHeader(500)
			return
		}
	case err := <-errCh:
		if respErr := SetErrorResponse("req", 400, err, w); respErr != nil {
			log.Errorf("ApiRequestHandler2: failed to set error response: %v", respErr)
			w.WriteHeader(500)
			return
		}
	}
}
