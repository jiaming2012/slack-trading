package models

// The reserved companion-stop tag prefix is enforced at the
// CreateOrderRequest.Validate seam so EVERY order-placement ingress
// (single-leg RPC, multi-leg RPC, REST) inherits the rejection
// (wire-companion-stops, re-review refinement 2).

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func validEquityRequest(tag string) *CreateOrderRequest {
	return &CreateOrderRequest{
		Symbol:         "AAPL",
		Class:          OrderRecordClassEquity,
		Quantity:       10,
		Side:           TradierOrderSideBuy,
		OrderType:      Market,
		Duration:       Day,
		RequestedPrice: 100.0,
		Tag:            tag,
	}
}

func TestCreateOrderRequestValidate_RejectsReservedCompanionStopTag(t *testing.T) {
	for _, tag := range []string{CompanionStopTag, CompanionStopTagForEntry(42)} {
		err := validEquityRequest(tag).Validate()
		require.Errorf(t, err, "tag %q must be rejected by request validation", tag)
		require.Contains(t, err.Error(), "reserved companion-stop prefix")
	}
}

func TestCreateOrderRequestValidate_AllowsOrdinaryTags(t *testing.T) {
	for _, tag := range []string{"", "strategy-entry", "companion-stopgap", "auto-closed-on-expiration", "exercise-call-option-7"} {
		require.NoErrorf(t, validEquityRequest(tag).Validate(), "tag %q must pass request validation", tag)
	}
}
