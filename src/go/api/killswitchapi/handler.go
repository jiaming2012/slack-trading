// Package killswitchapi exposes the operator REST surface for the hard kill
// switch: engage, release, acknowledge, and status. Every endpoint drives the
// single shared safety.HaltController that the Broker seam consults, so the REST
// surface and the automatic anomaly guards act on the exact same halt state.
package killswitchapi

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/go/backtester/safety"
)

// Handler holds the shared halt controller the endpoints operate on, plus the
// shared-secret token guarding the mutating endpoints (from KILL_SWITCH_TOKEN).
// An empty token means "not configured": engage stays available (stopping
// trading is the safe direction and must never be blocked by missing
// configuration) while the halt-weakening operations release and acknowledge
// are refused fail-closed.
type Handler struct {
	controller *safety.HaltController
	token      string
}

// NewHandler constructs a kill-switch REST handler bound to controller. token
// is the shared secret required on mutating endpoints; empty means not
// configured (fail-closed for release/acknowledge, open for engage).
func NewHandler(controller *safety.HaltController, token string) *Handler {
	return &Handler{controller: controller, token: token}
}

// presentedToken extracts the shared secret from the request: either
// "Authorization: Bearer <token>" or the "X-Kill-Switch-Token" header (curl
// ergonomics).
func presentedToken(r *http.Request) string {
	if auth := r.Header.Get("Authorization"); auth != "" {
		if strings.HasPrefix(auth, "Bearer ") {
			return strings.TrimPrefix(auth, "Bearer ")
		}
	}
	return r.Header.Get("X-Kill-Switch-Token")
}

// authorize enforces the token rules for a mutating endpoint. failClosed marks
// the halt-weakening operations (release/acknowledge) that must be refused
// when no token is configured. It writes the error response and returns false
// when the request is not permitted.
func (h *Handler) authorize(w http.ResponseWriter, r *http.Request, failClosed bool) bool {
	if h.token == "" {
		if failClosed {
			writeError(w, http.StatusForbidden, fmt.Errorf("kill-switch token is not configured: set KILL_SWITCH_TOKEN on the server to enable release/acknowledge (fail-closed)"))
			return false
		}
		// No token configured: engage stays available (safe direction).
		return true
	}

	presented := presentedToken(r)
	if subtle.ConstantTimeCompare([]byte(presented), []byte(h.token)) != 1 {
		writeError(w, http.StatusUnauthorized, fmt.Errorf("kill-switch token missing or invalid: pass 'Authorization: Bearer <token>' or 'X-Kill-Switch-Token: <token>'"))
		return false
	}
	return true
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
// optional reason from the request body. It requires the token when one is
// configured, but stays available when none is (stopping trading must never
// be blocked by missing configuration).
func (h *Handler) Engage(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r, false) {
		return
	}

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

// Release handles POST /kill-switch/release. It always requires a valid token
// (fail-closed when none is configured) and refuses with 409 Conflict when a
// cooldown acknowledgment is still outstanding.
func (h *Handler) Release(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r, true) {
		return
	}

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
// acknowledgment requirement of an auto-halt (without releasing the halt). As
// a halt-weakening operation it always requires a valid token (fail-closed
// when none is configured).
func (h *Handler) Acknowledge(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r, true) {
		return
	}

	if err := h.controller.Acknowledge(); err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
	log.Warnf("killswitchapi: auto-halt ACKNOWLEDGED")
	writeJSON(w, http.StatusOK, h.statusResponse())
}

// Status handles GET /kill-switch/status. It is deliberately unauthenticated:
// read-only, and needed by health tooling.
func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.statusResponse())
}

// SetupHandler registers the kill-switch routes on router (typically the
// "/kill-switch" subrouter). token is the shared secret from KILL_SWITCH_TOKEN
// (empty = not configured).
func SetupHandler(router *mux.Router, controller *safety.HaltController, token string) {
	h := NewHandler(controller, token)
	router.HandleFunc("/engage", h.Engage).Methods(http.MethodPost)
	router.HandleFunc("/release", h.Release).Methods(http.MethodPost)
	router.HandleFunc("/acknowledge", h.Acknowledge).Methods(http.MethodPost)
	router.HandleFunc("/status", h.Status).Methods(http.MethodGet)
}
