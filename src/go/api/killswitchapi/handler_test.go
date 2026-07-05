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

func muxTestRouter(c *safety.HaltController) *mux.Router {
	r := mux.NewRouter()
	SetupHandler(r.PathPrefix("/kill-switch").Subrouter(), c)
	return r
}

func newTestHandler(t *testing.T) *Handler {
	t.Helper()
	c, err := safety.NewHaltController(safety.NewMemoryHaltStore())
	require.NoError(t, err)
	return NewHandler(c)
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
	h := newTestHandler(t)
	require.NoError(t, h.controller.Engage("manual"))

	req := httptest.NewRequest(http.MethodPost, "/kill-switch/release", nil)
	rec := httptest.NewRecorder()
	h.Release(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	st := decodeStatus(t, rec.Body.Bytes())
	require.False(t, st.Engaged)
}

func TestRelease_BlockedWhileAckOutstanding(t *testing.T) {
	h := newTestHandler(t)
	require.NoError(t, h.controller.EngageAuto("guard"))

	req := httptest.NewRequest(http.MethodPost, "/kill-switch/release", nil)
	rec := httptest.NewRecorder()
	h.Release(rec, req)

	require.Equal(t, http.StatusConflict, rec.Code)
	require.True(t, h.controller.Status().Engaged, "auto-halt must stay engaged until acknowledged")
}

func TestAcknowledgeThenRelease_Resumes(t *testing.T) {
	h := newTestHandler(t)
	require.NoError(t, h.controller.EngageAuto("guard"))

	ack := httptest.NewRecorder()
	h.Acknowledge(ack, httptest.NewRequest(http.MethodPost, "/kill-switch/acknowledge", nil))
	require.Equal(t, http.StatusOK, ack.Code)
	require.False(t, decodeStatus(t, ack.Body.Bytes()).AckRequired)

	rel := httptest.NewRecorder()
	h.Release(rel, httptest.NewRequest(http.MethodPost, "/kill-switch/release", nil))
	require.Equal(t, http.StatusOK, rel.Code)
	require.False(t, h.controller.Status().Engaged)
}

func TestRoutesRegistered(t *testing.T) {
	c, err := safety.NewHaltController(safety.NewMemoryHaltStore())
	require.NoError(t, err)
	r := muxTestRouter(c)

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
