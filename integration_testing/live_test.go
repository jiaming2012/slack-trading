package integrationtesting

import (
	"context"
	"fmt"
	"testing"
)

func TestLiveAccount(t *testing.T) {
	ctx := context.Background()
	goEnv := "test"

	projectsDir, networkName := setupDatabases(t, ctx, goEnv)

	// Start main app container
	playgroundClient := createPlaygroundServerAndClient(ctx, t, projectsDir, networkName)

	fmt.Printf("Playground client: %v\n", playgroundClient)
}
