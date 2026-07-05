package telemetryapi

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jiaming2012/slack-trading/src/go/telemetry"
)

type upsertCall struct {
	kind string
	name string
	meta map[string]string
}

func newTestHandler(upsertErr error) (*Handler, *telemetry.HeartbeatTracker, *[]upsertCall) {
	tracker := telemetry.NewHeartbeatTracker()
	calls := &[]upsertCall{}
	h := &Handler{
		tracker: tracker,
		upsert: func(kind, name string, meta map[string]string, at time.Time) error {
			*calls = append(*calls, upsertCall{kind: kind, name: name, meta: meta})
			return upsertErr
		},
	}
	return h, tracker, calls
}

func post(h *Handler, body string) *httptest.ResponseRecorder {
	router := mux.NewRouter()
	sub := router.PathPrefix("/telemetry").Subrouter()
	sub.HandleFunc("/heartbeat", h.Heartbeat).Methods("POST")

	req := httptest.NewRequest(http.MethodPost, "/telemetry/heartbeat", strings.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestHeartbeat_ValidStrategyBeatIsRecorded(t *testing.T) {
	h, tracker, calls := newTestHandler(nil)

	rec := post(h, `{"kind":"strategy","name":"covered-call","meta":{"state":"active","tick_count":"42"}}`)

	assert.Equal(t, http.StatusOK, rec.Code)

	sources := tracker.Sources()
	require.Len(t, sources, 1)
	assert.Equal(t, "strategy", sources[0].Kind)
	assert.Equal(t, "covered-call", sources[0].Name)
	assert.Equal(t, "active", sources[0].Meta["state"])
	assert.Equal(t, int64(1), sources[0].BeatCount)

	require.Len(t, *calls, 1, "row upserted at ingest time")
	assert.Equal(t, "covered-call", (*calls)[0].name)
}

func TestHeartbeat_MissingNameRejectedWithoutSideEffects(t *testing.T) {
	h, tracker, calls := newTestHandler(nil)

	rec := post(h, `{"kind":"strategy"}`)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Empty(t, tracker.Sources())
	assert.Empty(t, *calls)
}

func TestHeartbeat_InvalidKindRejectedWithoutSideEffects(t *testing.T) {
	h, tracker, calls := newTestHandler(nil)

	rec := post(h, `{"kind":"server","name":"grodt"}`)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Empty(t, tracker.Sources())
	assert.Empty(t, *calls)
}

func TestHeartbeat_MalformedJSONRejected(t *testing.T) {
	h, tracker, _ := newTestHandler(nil)

	rec := post(h, `{not json`)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Empty(t, tracker.Sources())
}

func TestHeartbeat_UpsertFailureStillAcceptsBeat(t *testing.T) {
	h, tracker, _ := newTestHandler(errors.New("db down"))

	rec := post(h, `{"kind":"datasource","name":"polygon-options"}`)

	assert.Equal(t, http.StatusOK, rec.Code, "DB failure must not fail the beat")
	require.Len(t, tracker.Sources(), 1, "in-memory state still updated")
}

type fakeAcker struct {
	calls []struct {
		id  uint
		via string
	}
	err error
}

func (f *fakeAcker) Ack(id uint, via string, now time.Time) error {
	f.calls = append(f.calls, struct {
		id  uint
		via string
	}{id, via})
	return f.err
}

func postAck(h *Handler, path, body string) *httptest.ResponseRecorder {
	router := mux.NewRouter()
	sub := router.PathPrefix("/telemetry").Subrouter()
	sub.HandleFunc("/alerts/{id}/ack", h.AckAlert).Methods("POST")

	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestAckAlert_DefaultsToCLIChannel(t *testing.T) {
	acker := &fakeAcker{}
	h := &Handler{acker: acker}

	rec := postAck(h, "/telemetry/alerts/42/ack", "")

	assert.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, acker.calls, 1)
	assert.Equal(t, uint(42), acker.calls[0].id)
	assert.Equal(t, "cli", acker.calls[0].via)
}

func TestAckAlert_ViaFromBody(t *testing.T) {
	acker := &fakeAcker{}
	h := &Handler{acker: acker}

	rec := postAck(h, "/telemetry/alerts/7/ack", `{"via":"slack"}`)

	assert.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, acker.calls, 1)
	assert.Equal(t, "slack", acker.calls[0].via)
}

func TestAckAlert_UnknownAlertConflicts(t *testing.T) {
	acker := &fakeAcker{err: errors.New("alert #9 is not firing")}
	h := &Handler{acker: acker}

	rec := postAck(h, "/telemetry/alerts/9/ack", "")

	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), "not firing")
}

func TestAckAlert_NonNumericIDRejected(t *testing.T) {
	acker := &fakeAcker{}
	h := &Handler{acker: acker}

	rec := postAck(h, "/telemetry/alerts/abc/ack", "")

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Empty(t, acker.calls)
}

func TestHeartbeat_RepeatedBeatsIncrementCount(t *testing.T) {
	h, tracker, _ := newTestHandler(nil)

	post(h, `{"kind":"strategy","name":"covered-call"}`)
	post(h, `{"kind":"strategy","name":"covered-call"}`)

	sources := tracker.Sources()
	require.Len(t, sources, 1)
	assert.Equal(t, int64(2), sources[0].BeatCount)
}
