package slack

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

// AckFunc acknowledges a firing telemetry alert. Satisfied by
// telemetry.AlertEngine.Ack via a closure at setup time.
type AckFunc func(id uint, via string, now time.Time) error

// ackFn is the wired acknowledgement hook; nil until SetAckFunc is called.
var ackFn AckFunc

// SetAckFunc wires the alert-acknowledgement command. Called once from main.
func SetAckFunc(fn AckFunc) {
	ackFn = fn
}

// Handler receives Slack slash-command posts. Slack gives apps 3 seconds to
// respond; the ack command is fast enough to answer in-band, so the response
// body becomes the operator's feedback in Slack.
//
// Supported commands (from the slash command's text field):
//
//	ack <id>  — acknowledge telemetry alert <id>, silencing re-notification
func Handler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	text := strings.TrimSpace(r.Form.Get("text"))
	tokens := strings.Fields(text)

	if len(tokens) == 2 && tokens[0] == "ack" {
		handleAck(w, tokens[1])
		return
	}

	// Unknown or empty command: keep the historical 200-and-ignore behavior
	// so a misconfigured slash command never errors into the channel.
	w.WriteHeader(http.StatusOK)
}

func handleAck(w http.ResponseWriter, idRaw string) {
	respond := func(msg string) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(msg)); err != nil {
			log.Warnf("slack: failed to write ack response: %v", err)
		}
	}

	if ackFn == nil {
		respond("alert acknowledgement is not wired on this server")
		return
	}

	id, err := strconv.ParseUint(idRaw, 10, 64)
	if err != nil {
		respond(fmt.Sprintf("ack: %q is not an alert id — usage: ack <id>", idRaw))
		return
	}

	if err := ackFn(uint(id), "slack", time.Now().UTC()); err != nil {
		respond(fmt.Sprintf("ack failed: %v", err))
		return
	}

	respond(fmt.Sprintf("✅ alert #%d acknowledged — re-notification silenced while it stays firing", id))
}
