package killswitchapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"

	"github.com/jiaming2012/slack-trading/src/go/backtester/safety"
)

const testToken = "shhh-operator-secret"

func muxTestRouter(c *safety.HaltController, token string) *mux.Router {
	r := mux.NewRouter()
	SetupHandler(r.PathPrefix("/kill-switch").Subrouter(), c, token)
	return r
}

// newTestHandler builds a handler with NO token configured (legacy posture:
// engage open, release/acknowledge fail-closed).
func newTestHandler(t *testing.T) *Handler {
	t.Helper()
	return newTestHandlerWithToken(t, "")
}

func newTestHandlerWithToken(t *testing.T, token string) *Handler {
	t.Helper()
	c, err := safety.NewHaltController(safety.NewMemoryHaltStore())
	require.NoError(t, err)
	return NewHandler(c, token)
}

func withBearer(req *http.Request, token string) *http.Request {
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

func decodeStatus(t *testing.T, body []byte) statusResponse {
	t.Helper()
	var st statusResponse
	require.NoError(t, json.Unmarshal(body, &st))
	return st
}

func TestEngage_SetsEngagedManual(t *testing.T) {
	h := newTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/kill-switch/engage", strings.NewReader(`{"reason":"operator halt"}`))
	rec := httptest.NewRecorder()
	h.Engage(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	st := decodeStatus(t, rec.Body.Bytes())
	require.True(t, st.Engaged)
	require.Equal(t, "manual", st.Source)
	require.Equal(t, "operator halt", st.Reason)
	require.False(t, st.AckRequired)
}

func TestStatus_ReportsStateReasonSourceAck(t *testing.T) {
	h := newTestHandler(t)
	require.NoError(t, h.controller.EngageAuto("rejection-rate guard"))

	req := httptest.NewRequest(http.MethodGet, "/kill-switch/status", nil)
	rec := httptest.NewRecorder()
	h.Status(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	st := decodeStatus(t, rec.Body.Bytes())
	require.True(t, st.Engaged)
	require.Equal(t, "auto", st.Source)
	require.Equal(t, "rejection-rate guard", st.Reason)
	require.True(t, st.AckRequired)
}

func TestRelease_ClearsWhenNoAckOutstanding(t *testing.T) {
	h := newTestHandlerWithToken(t, testToken)
	require.NoError(t, h.controller.Engage("manual"))

	req := withBearer(httptest.NewRequest(http.MethodPost, "/kill-switch/release", nil), testToken)
	rec := httptest.NewRecorder()
	h.Release(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	st := decodeStatus(t, rec.Body.Bytes())
	require.False(t, st.Engaged)
}

func TestRelease_BlockedWhileAckOutstanding(t *testing.T) {
	h := newTestHandlerWithToken(t, testToken)
	require.NoError(t, h.controller.EngageAuto("guard"))

	req := withBearer(httptest.NewRequest(http.MethodPost, "/kill-switch/release", nil), testToken)
	rec := httptest.NewRecorder()
	h.Release(rec, req)

	require.Equal(t, http.StatusConflict, rec.Code)
	require.True(t, h.controller.Status().Engaged, "auto-halt must stay engaged until acknowledged")
}

func TestAcknowledgeThenRelease_Resumes(t *testing.T) {
	h := newTestHandlerWithToken(t, testToken)
	require.NoError(t, h.controller.EngageAuto("guard"))

	ack := httptest.NewRecorder()
	h.Acknowledge(ack, withBearer(httptest.NewRequest(http.MethodPost, "/kill-switch/acknowledge", nil), testToken))
	require.Equal(t, http.StatusOK, ack.Code)
	require.False(t, decodeStatus(t, ack.Body.Bytes()).AckRequired)

	rel := httptest.NewRecorder()
	h.Release(rel, withBearer(httptest.NewRequest(http.MethodPost, "/kill-switch/release", nil), testToken))
	require.Equal(t, http.StatusOK, rel.Code)
	require.False(t, h.controller.Status().Engaged)
}

func TestRoutesRegistered(t *testing.T) {
	c, err := safety.NewHaltController(safety.NewMemoryHaltStore())
	require.NoError(t, err)
	r := muxTestRouter(c, "")

	// GET /status is wired.
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/kill-switch/status", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	// POST /engage is wired and engages.
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/kill-switch/engage", strings.NewReader(`{}`)))
	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, c.Status().Engaged)
}

// --- Token authentication ---

func TestAuth_ValidTokenSucceedsForAllMutatingEndpoints(t *testing.T) {
	h := newTestHandlerWithToken(t, testToken)

	// Engage with token.
	rec := httptest.NewRecorder()
	h.Engage(rec, withBearer(httptest.NewRequest(http.MethodPost, "/kill-switch/engage", strings.NewReader(`{"reason":"drill"}`)), testToken))
	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, h.controller.Status().Engaged)

	// Escalate to auto so acknowledge is meaningful.
	require.NoError(t, h.controller.EngageAuto("guard"))

	// Acknowledge with token (X-Kill-Switch-Token header form).
	rec = httptest.NewRecorder()
	ackReq := httptest.NewRequest(http.MethodPost, "/kill-switch/acknowledge", nil)
	ackReq.Header.Set("X-Kill-Switch-Token", testToken)
	h.Acknowledge(rec, ackReq)
	require.Equal(t, http.StatusOK, rec.Code)
	require.False(t, h.controller.Status().AckRequired)

	// Release with token.
	rec = httptest.NewRecorder()
	h.Release(rec, withBearer(httptest.NewRequest(http.MethodPost, "/kill-switch/release", nil), testToken))
	require.Equal(t, http.StatusOK, rec.Code)
	require.False(t, h.controller.Status().Engaged)
}

func TestAuth_MissingOrInvalidTokenRejectedWithStateUnchanged(t *testing.T) {
	cases := []struct {
		name  string
		token string // presented token; empty = header absent
	}{
		{name: "missing token", token: ""},
		{name: "invalid token", token: "wrong-secret"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHandlerWithToken(t, testToken)
			require.NoError(t, h.controller.EngageAuto("guard"))
			before := h.controller.Status()

			for _, ep := range []struct {
				name string
				call func(w http.ResponseWriter, r *http.Request)
			}{
				{name: "engage", call: h.Engage},
				{name: "release", call: h.Release},
				{name: "acknowledge", call: h.Acknowledge},
			} {
				req := httptest.NewRequest(http.MethodPost, "/kill-switch/"+ep.name, nil)
				if tc.token != "" {
					req = withBearer(req, tc.token)
				}
				rec := httptest.NewRecorder()
				ep.call(rec, req)

				require.Equal(t, http.StatusUnauthorized, rec.Code, "%s must reject a %s", ep.name, tc.name)
				require.Equal(t, before, h.controller.Status(), "%s with %s must not change controller state", ep.name, tc.name)
			}
		})
	}
}

func TestAuth_ReleaseAndAcknowledgeFailClosedWhenNoTokenConfigured(t *testing.T) {
	h := newTestHandler(t) // no token configured
	require.NoError(t, h.controller.EngageAuto("guard"))

	rec := httptest.NewRecorder()
	h.Acknowledge(rec, httptest.NewRequest(http.MethodPost, "/kill-switch/acknowledge", nil))
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), "KILL_SWITCH_TOKEN")
	require.True(t, h.controller.Status().AckRequired, "acknowledge must be refused fail-closed")

	rec = httptest.NewRecorder()
	h.Release(rec, httptest.NewRequest(http.MethodPost, "/kill-switch/release", nil))
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), "KILL_SWITCH_TOKEN")
	require.True(t, h.controller.Status().Engaged, "controller must remain engaged when release is refused")
}

func TestAuth_EngageOpenWhenNoTokenConfigured(t *testing.T) {
	h := newTestHandler(t) // no token configured

	rec := httptest.NewRecorder()
	h.Engage(rec, httptest.NewRequest(http.MethodPost, "/kill-switch/engage", strings.NewReader(`{"reason":"halt now"}`)))
	require.Equal(t, http.StatusOK, rec.Code, "engage (the safe direction) must never be blocked by missing token configuration")
	require.True(t, h.controller.Status().Engaged)
}

func TestAuth_StatusNeedsNoToken(t *testing.T) {
	c, err := safety.NewHaltController(safety.NewMemoryHaltStore())
	require.NoError(t, err)
	r := muxTestRouter(c, testToken)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/kill-switch/status", nil))
	require.Equal(t, http.StatusOK, rec.Code)
}
