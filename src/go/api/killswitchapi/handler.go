// Package killswitchapi exposes the operator REST surface for the hard kill
// switch: engage, release, acknowledge, and status. Every endpoint drives the
// single shared safety.HaltController that the Broker seam consults, so the REST
// surface and the automatic anomaly guards act on the exact same halt state.
package killswitchapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/go/backtester/safety"
)

// Handler holds the shared halt controller the endpoints operate on.
type Handler struct {
	controller *safety.HaltController
}

// NewHandler constructs a kill-switch REST handler bound to controller.
func NewHandler(controller *safety.HaltController) *Handler {
	return &Handler{controller: controller}
}

type engageRequest struct {
	Reason string `json:"reason"`
}

// statusResponse is the shape returned by every endpoint, so an operator (or the
// Taskfile wrapper) always sees the resulting halt state.
type statusResponse struct {
	Engaged     bool   `json:"engaged"`
	Reason      string `json:"reason"`
	Source      string `json:"source"`
	AckRequired bool   `json:"ack_required"`
}

func (h *Handler) statusResponse() statusResponse {
	st := h.controller.Status()
	return statusResponse{
		Engaged:     st.Engaged,
		Reason:      st.Reason,
		Source:      string(st.Source),
		AckRequired: st.AckRequired,
	}
}

func writeJSON(w http.ResponseWriter, code int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Errorf("killswitchapi: encode response: %v", err)
	}
}

func writeError(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]string{"error": err.Error()})
}

// Engage handles POST /kill-switch/engage. It engages a manual halt with an
// optional reason from the request body.
func (h *Handler) Engage(w http.ResponseWriter, r *http.Request) {
	reason := "manual kill switch engaged"
	if r.Body != nil {
		var req engageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil && req.Reason != "" {
			reason = req.Reason
		}
	}

	if err := h.controller.Engage(reason); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	log.Warnf("killswitchapi: kill switch ENGAGED (manual): %s", reason)
	writeJSON(w, http.StatusOK, h.statusResponse())
}

// Release handles POST /kill-switch/release. It refuses with 409 Conflict when a
// cooldown acknowledgment is still outstanding.
func (h *Handler) Release(w http.ResponseWriter, r *http.Request) {
	if err := h.controller.Release(); err != nil {
		if errors.Is(err, safety.ErrAckRequired) {
			writeJSON(w, http.StatusConflict, map[string]interface{}{
				"error":  err.Error(),
				"status": h.statusResponse(),
			})
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	log.Warnf("killswitchapi: kill switch RELEASED")
	writeJSON(w, http.StatusOK, h.statusResponse())
}

// Acknowledge handles POST /kill-switch/acknowledge. It clears the cooldown
// acknowledgment requirement of an auto-halt (without releasing the halt).
func (h *Handler) Acknowledge(w http.ResponseWriter, r *http.Request) {
	if err := h.controller.Acknowledge(); err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
	log.Warnf("killswitchapi: auto-halt ACKNOWLEDGED")
	writeJSON(w, http.StatusOK, h.statusResponse())
}

// Status handles GET /kill-switch/status.
func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.statusResponse())
}

// SetupHandler registers the kill-switch routes on router (typically the
// "/kill-switch" subrouter).
func SetupHandler(router *mux.Router, controller *safety.HaltController) {
	h := NewHandler(controller)
	router.HandleFunc("/engage", h.Engage).Methods(http.MethodPost)
	router.HandleFunc("/release", h.Release).Methods(http.MethodPost)
	router.HandleFunc("/acknowledge", h.Acknowledge).Methods(http.MethodPost)
	router.HandleFunc("/status", h.Status).Methods(http.MethodGet)
}
