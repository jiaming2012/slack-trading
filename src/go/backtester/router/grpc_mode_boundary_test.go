package router

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	pb "github.com/jiaming2012/slack-trading/src/go/playground"
)

// newModeBoundaryTestServer builds a Server with NO database service wired.
// The mode-resolution boundary runs before any database or broker access, so
// a request rejected at the boundary must return an error without touching
// dbService — if the handler proceeded past the boundary it would panic on
// the nil service, failing the test.
func newModeBoundaryTestServer() *Server {
	return &Server{}
}

// TestCreateLivePlayground_RejectsContradictoryCombination: the crossed pair
// (environment="live", account_type="simulator") corresponds to no mode
// preset and must be rejected with no playground created
// (legacy-enum-compat-mapping spec scenario).
func TestCreateLivePlayground_RejectsContradictoryCombination(t *testing.T) {
	s := newModeBoundaryTestServer()

	resp, err := s.CreateLivePlayground(context.Background(), &pb.CreateLivePlaygroundRequest{
		Balance:     1000,
		Broker:      "tradier",
		Environment: "live",
		AccountType: "simulator",
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "no mode preset")
	require.Nil(t, resp)
}

// TestCreateLivePlayground_RejectsReconcile: reconcile is not an
// operator-selectable mode (reconcile-concept-retirement).
func TestCreateLivePlayground_RejectsReconcile(t *testing.T) {
	s := newModeBoundaryTestServer()

	resp, err := s.CreateLivePlayground(context.Background(), &pb.CreateLivePlaygroundRequest{
		Balance:     1000,
		Broker:      "tradier",
		Environment: "reconcile",
		AccountType: "reconcilation",
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "not an operator-selectable mode")
	require.Nil(t, resp)
}

// TestCreateLivePlayground_ValidCombinationResolvesToMode: a valid
// (live, paper) request passes the mode boundary. Without Tradier env vars
// configured the handler then fails on account-id resolution — an error that
// proves the request resolved to a mode and proceeded past the boundary.
func TestCreateLivePlayground_ValidCombinationResolvesToMode(t *testing.T) {
	s := newModeBoundaryTestServer()

	resp, err := s.CreateLivePlayground(context.Background(), &pb.CreateLivePlaygroundRequest{
		Balance:     1000,
		Broker:      "tradier",
		Environment: "live",
		AccountType: "paper",
	})

	require.Error(t, err)
	require.NotContains(t, err.Error(), "no mode preset", "a valid (live, paper) pair must resolve to a mode")
	require.NotContains(t, err.Error(), "not an operator-selectable mode")
	require.Nil(t, resp)
}

// TestCreatePlayground_RejectsNonSimulationEnvironments: the polygon
// CreatePlayground entry point carries no account source, so only
// environment="simulator" resolves; "live" has no preset without an account
// type and "reconcile" is not operator-selectable.
func TestCreatePlayground_RejectsNonSimulationEnvironments(t *testing.T) {
	s := newModeBoundaryTestServer()

	resp, err := s.CreatePlayground(context.Background(), &pb.CreatePolygonPlaygroundRequest{
		Balance:     1000,
		Environment: "live",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "no mode preset")
	require.Nil(t, resp)

	resp, err = s.CreatePlayground(context.Background(), &pb.CreatePolygonPlaygroundRequest{
		Balance:     1000,
		Environment: "reconcile",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not an operator-selectable mode")
	require.Nil(t, resp)
}
