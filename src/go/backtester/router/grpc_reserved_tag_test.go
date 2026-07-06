package router

// Boundary tests for the reserved companion-stop tag prefix
// (wire-companion-stops, adversarial-review finding 3a): a client-supplied
// order tagged with the reserved prefix could poison the companion-stop
// eligibility check (suppressing a protective stop for a real entry) or the
// restart-window idempotency lookup, so the PlaceOrder RPC rejects it with an
// invalid-argument (4xx) error before any database or broker access. Like the
// mode-boundary tests, the Server carries no database service: a request that
// proceeded past the boundary would panic on the nil service.

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/twitchtv/twirp"

	pb "github.com/jiaming2012/slack-trading/src/go/playground"
)

func placeOrderRequestWithTag(tag string) *pb.PlaceOrderRequest {
	return &pb.PlaceOrderRequest{
		PlaygroundId:   "00000000-0000-0000-0000-000000000001",
		Symbol:         "AAPL",
		AssetClass:     "equity",
		Quantity:       10,
		Side:           "buy",
		Type:           "market",
		Duration:       "day",
		RequestedPrice: 100.0,
		Tag:            tag,
	}
}

func TestPlaceOrder_RejectsReservedCompanionStopTag(t *testing.T) {
	s := &Server{}

	for _, tag := range []string{"companion-stop", "companion-stop-42"} {
		resp, err := s.PlaceOrder(context.Background(), placeOrderRequestWithTag(tag))
		require.Errorf(t, err, "tag %q must be rejected at the RPC boundary", tag)
		require.Nil(t, resp)

		var twErr twirp.Error
		require.True(t, errors.As(err, &twErr), "the rejection must be a Twirp error so the client sees a 4xx")
		require.Equal(t, twirp.InvalidArgument, twErr.Code())
		require.Contains(t, twErr.Msg(), "reserved companion-stop prefix")
	}
}

// The multi-leg ingress inherits the reserved-tag rejection through
// CreateOrderRequest.Validate: every leg request built by
// buildMultiLegRequests carries its leg's client-supplied tag, and validation
// (run by dbService.PlaceOrders on every request) refuses the reserved
// prefix.
func TestPlaceMultiLegOrder_LegRequestsInheritReservedTagRejection(t *testing.T) {
	requests := buildMultiLegRequests(&pb.PlaceMultiLegOrderRequest{
		PlaygroundId: "00000000-0000-0000-0000-000000000001",
		Type:         "market",
		Duration:     "day",
		Legs: []*pb.MultiLegOrderLeg{
			{Symbol: "O:AAPL250703C00210000", AssetClass: "option", Quantity: 1, Side: "sell_to_open", Tag: "companion-stop-42"},
			{Symbol: "O:AAPL250703C00215000", AssetClass: "option", Quantity: 1, Side: "buy_to_open", Tag: "spread-hedge"},
		},
	})
	require.Len(t, requests, 2)

	err := requests[0].Validate()
	require.Error(t, err, "a leg carrying the reserved tag must be rejected by request validation")
	require.Contains(t, err.Error(), "reserved companion-stop prefix")

	require.NoError(t, requests[1].Validate(), "ordinary leg tags must pass")
}

func TestPlaceOrder_AllowsOrdinaryTagsPastTheBoundary(t *testing.T) {
	s := &Server{}

	for _, tag := range []string{"", "strategy-entry", "companion-stopgap"} {
		// With no database service wired, an allowed tag proceeds past the
		// boundary and panics on the nil service — proving the tag check did
		// not reject it. A rejection would return cleanly instead.
		require.Panicsf(t, func() {
			_, _ = s.PlaceOrder(context.Background(), placeOrderRequestWithTag(tag))
		}, "tag %q must pass the reserved-tag boundary", tag)
	}
}
