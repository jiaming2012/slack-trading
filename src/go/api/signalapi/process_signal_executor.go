package signalapi

import (
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"

	"github.com/jiaming2012/slack-trading/src/go/models"
	"github.com/jiaming2012/slack-trading/src/go/api"
)

type ProcessSignalExecutor struct {
	esdbProducer *api.EsdbProducer
}

func (s *ProcessSignalExecutor) Serve(r *http.Request, request models.ApiRequest3, resultCh chan interface{}, errCh chan error) {
	tracer := otel.Tracer("ProcessSignalExecutor")
	ctx, span := tracer.Start(r.Context(), "ProcessSignalExecutor.Serve")
	defer span.End()

	logger := log.WithContext(ctx)

	req, ok := request.(*models.CreateSignalRequestEventV1DTO)
	if !ok {
		errCh <- models.ErrInvalidRequestType
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

func NewProcessSignalExecutor(esdbProducer *api.EsdbProducer) *ProcessSignalExecutor {
	return &ProcessSignalExecutor{
		esdbProducer: esdbProducer,
	}
}
