// Package telemetryapi exposes the REST ingestion surface for the in-process
// Telemetry module (ADR-0005): strategies and datasources post their
// heartbeats here, and operators acknowledge alerts here.
package telemetryapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/jiaming2012/slack-trading/src/go/telemetry"
)

// upsertFunc persists one heartbeat row; injected so tests run without a DB.
type upsertFunc func(kind, name string, meta map[string]string, at time.Time) error

// Acker acknowledges a firing alert. Satisfied by *telemetry.AlertEngine.
type Acker interface {
	Ack(id uint, via string, now time.Time) error
}

// Handler binds the heartbeat tracker, the persistence hook, and the alert
// engine's ack surface.
type Handler struct {
	tracker *telemetry.HeartbeatTracker
	upsert  upsertFunc
	acker   Acker
}

func NewHandler(db *gorm.DB, tracker *telemetry.HeartbeatTracker, acker Acker) *Handler {
	return &Handler{
		tracker: tracker,
		upsert: func(kind, name string, meta map[string]string, at time.Time) error {
			return telemetry.UpsertHeartbeat(db, kind, name, meta, at)
		},
		acker: acker,
	}
}

type heartbeatRequest struct {
	Kind string            `json:"kind"`
	Name string            `json:"name"`
	Meta map[string]string `json:"meta"`
}

func writeJSON(w http.ResponseWriter, code int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Errorf("telemetryapi: encode response: %v", err)
	}
}

func writeError(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]string{"error": err.Error()})
}

// Heartbeat handles POST /telemetry/heartbeat. A valid beat updates the
// in-memory tracker and upserts the source's telemetry_heartbeats row; an
// invalid payload is rejected with 400 and no side effects.
func (h *Handler) Heartbeat(w http.ResponseWriter, r *http.Request) {
	var req heartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errors.New("invalid JSON body"))
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, errors.New("name is required"))
		return
	}
	if !telemetry.ValidSourceKind(req.Kind) {
		writeError(w, http.StatusBadRequest, errors.New(`kind must be "strategy" or "datasource"`))
		return
	}

	now := time.Now().UTC()
	h.tracker.Beat(req.Kind, req.Name, req.Meta, now)

	// A DB hiccup must not fail the beat: the in-memory state is what the
	// alert rules evaluate; the row catches up on the next beat.
	if err := h.upsert(req.Kind, req.Name, req.Meta, now); err != nil {
		log.Warnf("telemetryapi: heartbeat upsert failed for %s/%s: %v", req.Kind, req.Name, err)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type ackRequest struct {
	Via string `json:"via"`
}

// AckAlert handles POST /telemetry/alerts/{id}/ack. Unknown or already-
// resolved ids return an error with no side effects.
func (h *Handler) AckAlert(w http.ResponseWriter, r *http.Request) {
	idRaw := mux.Vars(r)["id"]
	id, err := strconv.ParseUint(idRaw, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, errors.New("alert id must be a positive integer"))
		return
	}

	via := telemetry.AckViaCLI
	var req ackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err == nil && req.Via != "" {
		via = req.Via
	}

	if err := h.acker.Ack(uint(id), via, time.Now().UTC()); err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "acknowledged", "id": id, "via": via})
}

// SetupHandler registers the telemetry routes on the given subrouter.
func SetupHandler(router *mux.Router, db *gorm.DB, tracker *telemetry.HeartbeatTracker, acker Acker) *Handler {
	h := NewHandler(db, tracker, acker)
	router.HandleFunc("/heartbeat", h.Heartbeat).Methods("POST")
	router.HandleFunc("/alerts/{id}/ack", h.AckAlert).Methods("POST")
	return h
}
