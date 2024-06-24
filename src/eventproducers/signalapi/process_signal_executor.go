package signalapi

import (
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/eventproducers"
)

type ProcessSignalExecutor struct {
	esdbProducer *eventproducers.EsdbProducer
}

func (s *ProcessSignalExecutor) Serve(r *http.Request, request eventmodels.ApiRequest3, resultCh chan map[string]interface{}, errCh chan error) {
	tracer := otel.Tracer("ProcessSignalExecutor")
	ctx, span := tracer.Start(r.Context(), "ProcessSignalExecutor.Serve")
	defer span.End()

	logger := log.WithContext(ctx)
	
	req, ok := request.(*eventmodels.CreateSignalRequestEventV1DTO)
	if !ok {
		errCh <- eventmodels.ErrInvalidRequestType
		return
	}

	if ok, err := s.esdbProducer.ProcessSaveCreateSignalRequestEvent(ctx, req); err != nil {
		if ok {
			logger.WithFields(log.Fields{
				"event": "signal",
			}).Debugf("handleSaveCreateSignalRequestEvent: %v", err)
			span.RecordError(err)
			span.SetStatus(codes.Error, "potential failure occurred")
		} else {
			errCh <- fmt.Errorf("failed to process save create signal request event: %w", err)
			return
		}
	}

	resultCh <- map[string]interface{}{}
}

func NewProcessSignalExecutor(esdbProducer *eventproducers.EsdbProducer) *ProcessSignalExecutor {
	return &ProcessSignalExecutor{
		esdbProducer: esdbProducer,
	}
}
