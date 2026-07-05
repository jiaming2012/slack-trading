package router

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	pb "github.com/jiaming2012/slack-trading/src/go/playground"
)

// TestPlaceMultiLegOrder_SpreadAttributes verifies that buildMultiLegRequests
// injects a shared spread_group_key UUID and correct leg_role into each leg's
// Attributes map.
func TestPlaceMultiLegOrder_SpreadAttributes(t *testing.T) {
	req := &pb.PlaceMultiLegOrderRequest{
		PlaygroundId: uuid.New().String(),
		Legs: []*pb.MultiLegOrderLeg{
			{
				Symbol:     "O:SPX250321C05000000",
				AssetClass: "option",
				Quantity:   1,
				Side:       "buy_to_open",
				Tag:        "long_call",
			},
			{
				Symbol:     "O:SPX250321C05100000",
				AssetClass: "option",
				Quantity:   1,
				Side:       "sell_to_open",
				Tag:        "short_call",
			},
		},
		Type:     "market",
		Duration: "day",
	}

	requests := buildMultiLegRequests(req)

	require.Len(t, requests, 2, "expected 2 order requests")

	// Both legs must share the same spread_group_key
	key0 := requests[0].Attributes["spread_group_key"]
	key1 := requests[1].Attributes["spread_group_key"]
	require.NotEmpty(t, key0, "spread_group_key must not be empty")
	require.Equal(t, key0, key1, "both legs must share the same spread_group_key")

	// spread_group_key must be a valid UUID
	_, err := uuid.Parse(key0)
	require.NoError(t, err, "spread_group_key must be a valid UUID")

	// leg_role: buy_to_open -> "long", sell_to_open -> "short"
	require.Equal(t, "long", requests[0].Attributes["leg_role"], "buy_to_open leg should have leg_role=long")
	require.Equal(t, "short", requests[1].Attributes["leg_role"], "sell_to_open leg should have leg_role=short")
}

// TestPlaceMultiLegOrder_LegRoleSellShort verifies that sell_short is mapped to "short"
// and buy is mapped to "long".
func TestPlaceMultiLegOrder_LegRoleSellShort(t *testing.T) {
	req := &pb.PlaceMultiLegOrderRequest{
		PlaygroundId: uuid.New().String(),
		Legs: []*pb.MultiLegOrderLeg{
			{
				Symbol:     "AAPL",
				AssetClass: "equity",
				Quantity:   100,
				Side:       "buy",
				Tag:        "long_equity",
			},
			{
				Symbol:     "AAPL",
				AssetClass: "equity",
				Quantity:   100,
				Side:       "sell_short",
				Tag:        "short_equity",
			},
		},
		Type:     "market",
		Duration: "day",
	}

	requests := buildMultiLegRequests(req)

	require.Equal(t, "long", requests[0].Attributes["leg_role"], "buy leg should have leg_role=long")
	require.Equal(t, "short", requests[1].Attributes["leg_role"], "sell_short leg should have leg_role=short")
}

// TestPlaceMultiLegOrder_UnknownSide verifies that unrecognized sides get leg_role="unknown".
func TestPlaceMultiLegOrder_UnknownSide(t *testing.T) {
	req := &pb.PlaceMultiLegOrderRequest{
		PlaygroundId: uuid.New().String(),
		Legs: []*pb.MultiLegOrderLeg{
			{
				Symbol:     "O:SPX250321C05000000",
				AssetClass: "option",
				Quantity:   1,
				Side:       "sell_to_close",
				Tag:        "close_leg",
			},
		},
		Type:     "market",
		Duration: "day",
	}

	requests := buildMultiLegRequests(req)

	require.Len(t, requests, 1)
	require.Equal(t, "unknown", requests[0].Attributes["leg_role"], "sell_to_close should have leg_role=unknown")
	require.NotEmpty(t, requests[0].Attributes["spread_group_key"], "spread_group_key must be set even for unknown roles")
}
