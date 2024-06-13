package eventmodels

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
)

type CreateSignalV1DTO struct {
	Header      SignalRequestHeader `json:"header"`
	Name        string              `json:"name"`
	LastUpdated time.Time           `json:"last_updated"`
	Plot0       *string             `json:"plot_0"`
	Plot1       *string             `json:"plot_1"`
}

func (dto *CreateSignalV1DTO) ToModel() (*CreateSignalV1, error) {
	var plot0, plot1 *float64

	if dto.Plot0 != nil {
		p0, err := strconv.ParseFloat(*dto.Plot0, 64)
		if err != nil {
			return nil, fmt.Errorf("CreateSignalRequestEventV1DTO.Validate: failed to parse plot0: %w", err)
		}

		plot0 = &p0
	}

	if dto.Plot1 != nil {
		p1, err := strconv.ParseFloat(*dto.Plot1, 64)
		if err != nil {
			return nil, fmt.Errorf("CreateSignalRequestEventV1DTO.Validate: failed to parse plot1: %w", err)
		}

		plot1 = &p1
	}

	return &CreateSignalV1{
		Header: dto.Header,
		Name:   dto.Name,
		Plot0:  plot0,
		Plot1:  plot1,
	}, nil
}

type CreateSignalV1 struct {
	Header SignalRequestHeader
	Name   string
	Plot0  *float64
	Plot1  *float64
}

// todo: deprecated for event models
type CreateSignalRequestEventV1DTO struct {
	BaseRequestEvent
	CreateSignalV1DTO
}

func (dto *CreateSignalRequestEventV1DTO) ValidateV2() (bool, error) {
	signal, err := dto.CreateSignalV1DTO.ToModel()
	if err != nil {
		return false, fmt.Errorf("CreateSignalRequestEventV1DTO.Convert: failed to convert CreateSignalV1DTO to model: %w", err)
	}

	switch signal.Name {
	case "stochastic_rsi-buy":
		if signal.Plot0 == nil {
			return false, fmt.Errorf("CreateSignalRequestEventV1DTO.Validate: plot0 was not set for stochastic_rsi-buy")
		}

		if signal.Plot1 == nil {
			return false, fmt.Errorf("CreateSignalRequestEventV1DTO.Validate: plot1 was not set for stochastic_rsi-buy")
		}

		if K, _ := *signal.Plot0, *signal.Plot1; K > 20 {
			return true, fmt.Errorf("CreateSignalRequestEventV1DTO.Validate: K was greater than 20 for stochastic_rsi-buy")
		}

	case "stochastic_rsi-sell":
		if signal.Plot0 == nil {
			return false, fmt.Errorf("CreateSignalRequestEventV1DTO.Validate: plot0 was not set for stochastic_rsi-sell")
		}

		if signal.Plot1 == nil {
			return false, fmt.Errorf("CreateSignalRequestEventV1DTO.Validate: plot1 was not set for stochastic_rsi-sell")
		}

		if K, _ := *signal.Plot0, *signal.Plot1; K < 80 {
			return true, fmt.Errorf("CreateSignalRequestEventV1DTO.Validate: K was less than 80 for stochastic_rsi-sell")
		}
	}

	return true, nil
}

type CreateSignalRequestEventV1 struct {
	Header SignalRequestHeader
	Signal CreateSignalV1
}

func (r *CreateSignalRequestEventV1DTO) GetSavedEventParameters() SavedEventParameters {
	return SavedEventParameters{
		StreamName:    AccountsStream,
		EventName:     CreateSignalRequestEventName,
		SchemaVersion: 1,
	}
}

func (r *CreateSignalRequestEventV1DTO) ConvertToTracker(now time.Time) (*TrackerV2, error) {
	symbol := StockSymbol(r.Header.Symbol)
	if symbol == "" {
		return nil, fmt.Errorf("CreateSignalRequestEvent.ConvertToTracker: symbol was not set")
	}

	return NewSignalTrackerV2(r.Name, r.Header, now, r.GetMetaData().RequestID), nil
}

func NewSignalRequest(requestID uuid.UUID, name string) *CreateSignalRequestEventV1DTO {
	request := &CreateSignalRequestEventV1DTO{CreateSignalV1DTO: CreateSignalV1DTO{Name: name}}
	request.SetMetaData(&MetaData{RequestID: requestID})

	return request
}

func (r *CreateSignalRequestEventV1DTO) String() string {
	return fmt.Sprintf("SignalRequest: %v, source=%v", r.Name, r.Header.Source)
}

func (r *CreateSignalRequestEventV1DTO) GetSource() SignalSource {
	return r.Header.Source
}

func (r *CreateSignalRequestEventV1DTO) Validate(req *http.Request) error {
	if r.Name == "" {
		return fmt.Errorf("SignalRequest.Validate: name was not set")
	}

	return nil
}

func (r *CreateSignalRequestEventV1DTO) ParseHTTPRequest(req *http.Request) error {
	if err := json.NewDecoder(req.Body).Decode(r); err != nil {
		return fmt.Errorf("SignalRequest.ParseHTTPRequest: failed to unmarshal request body: %w", err)
	}

	if r.Name == "" {
		return fmt.Errorf("SignalRequest.ParseHTTPRequest: account name was not found")
	}

	return nil
}
