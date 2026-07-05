// Package telemetryapi exposes the REST ingestion surface for the in-process
// Telemetry module (ADR-0005): strategies and datasources post their
// heartbeats here, and operators acknowledge alerts here.
package telemetryapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/jiaming2012/slack-trading/src/go/telemetry"
)

// upsertFunc persists one heartbeat row; injected so tests run without a DB.
type upsertFunc func(kind, name string, meta map[string]string, at time.Time) error

// Handler binds the heartbeat tracker and the persistence hook.
type Handler struct {
	tracker *telemetry.HeartbeatTracker
	upsert  upsertFunc
}

func NewHandler(db *gorm.DB, tracker *telemetry.HeartbeatTracker) *Handler {
	return &Handler{
		tracker: tracker,
		upsert: func(kind, name string, meta map[string]string, at time.Time) error {
			return telemetry.UpsertHeartbeat(db, kind, name, meta, at)
		},
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

// SetupHandler registers the telemetry routes on the given subrouter.
func SetupHandler(router *mux.Router, db *gorm.DB, tracker *telemetry.HeartbeatTracker) *Handler {
	h := NewHandler(db, tracker)
	router.HandleFunc("/heartbeat", h.Heartbeat).Methods("POST")
	return h
}
