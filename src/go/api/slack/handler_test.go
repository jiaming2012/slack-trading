package slack

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func postCommand(text string) *httptest.ResponseRecorder {
	form := url.Values{"text": {text}}
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	Handler(rec, req)
	return rec
}

func TestHandler_AckCommandAcknowledges(t *testing.T) {
	var gotID uint
	var gotVia string
	SetAckFunc(func(id uint, via string, now time.Time) error {
		gotID = id
		gotVia = via
		return nil
	})
	defer SetAckFunc(nil)

	rec := postCommand("ack 42")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, uint(42), gotID)
	assert.Equal(t, "slack", gotVia)
	assert.Contains(t, rec.Body.String(), "#42 acknowledged")
}

func TestHandler_AckFailureReportsInBand(t *testing.T) {
	SetAckFunc(func(id uint, via string, now time.Time) error {
		return errors.New("alert #9 is not firing")
	})
	defer SetAckFunc(nil)

	rec := postCommand("ack 9")

	assert.Equal(t, http.StatusOK, rec.Code, "Slack requires 200 within 3s; errors go in the body")
	assert.Contains(t, rec.Body.String(), "not firing")
}

func TestHandler_AckBadIDExplainsUsage(t *testing.T) {
	called := false
	SetAckFunc(func(id uint, via string, now time.Time) error {
		called = true
		return nil
	})
	defer SetAckFunc(nil)

	rec := postCommand("ack forty-two")

	require.False(t, called)
	assert.Contains(t, rec.Body.String(), "usage: ack <id>")
}

func TestHandler_UnknownCommandStaysSilent(t *testing.T) {
	rec := postCommand("balance btc")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Body.String())
}
