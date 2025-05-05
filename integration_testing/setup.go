package integrationtesting

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/jiaming2012/slack-trading/src/playground"
	"github.com/jiaming2012/slack-trading/src/utils"
)

// LogConsumerCfg is a configuration for a log consumer
type LogConsumer struct{}

func (c *LogConsumer) Accept(l testcontainers.Log) {
	// if l.LogType == testcontainers.StdoutLog {
	fmt.Println(string(l.Content))
	// }
}

func createPlaygroundServerAndClient(ctx context.Context, t *testing.T, projectsDir, networkName string) playground.PlaygroundService {
	logConsumer := &LogConsumer{}

	appContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "ewr.vultrcr.com/grodt/app:latest-dev",
			ExposedPorts: []string{"5051/tcp"},
			Env: map[string]string{
				"PROJECTS_DIR":     "/app",
				"GO_ENV":           "test",
				"DRY_RUN":          "false",
				"POSTGRES_HOST":    "postgres",
				"POSTGRES_PORT":    "5432",
				"ANACONDA_HOME":    "/opt/conda",
				"EVENTSTOREDB_URL": "esdb://admin:changeit@eventstoredb:2113?tls=false&keepAliveTimeout=10000&keepAliveInterval=10000",
			},
			WaitingFor: wait.ForAll(
				wait.ForExposedPort(),
				wait.ForListeningPort("5051/tcp").WithStartupTimeout(30*time.Second),
				wait.ForLog("Main: init complete"),
			),
			Files: []testcontainers.ContainerFile{
				{
					HostFilePath:      filepath.Join(projectsDir, "slack-trading", ".env"),
					ContainerFilePath: "/app/slack-trading/.env",
					FileMode:          0644,
				},
			},
			Networks:       []string{networkName},
			LogConsumerCfg: &testcontainers.LogConsumerConfig{Consumers: []testcontainers.LogConsumer{logConsumer}},
		},
		Started: true,
	})
	testcontainers.CleanupContainer(t, appContainer)

	require.NoError(t, err)

	// Create a Playground
	appContainerHost, err := appContainer.Host(ctx)
	require.NoError(t, err)

	appContainerPort, err := appContainer.MappedPort(ctx, "5051/tcp")
	require.NoError(t, err)

	twirpUrl := fmt.Sprintf("http://%s:%s", appContainerHost, appContainerPort.Port())

	client := http.Client{
		Timeout: 30 * time.Second,
	}

	playgroundClient := playground.NewPlaygroundServiceProtobufClient(twirpUrl, &client)

	return playgroundClient
}

func setupDatabases(t *testing.T, ctx context.Context, goEnv string) (projectsDir, networkName string) {
	err := godotenv.Load()
	if err != nil {
		dir, _ := os.Getwd()
		log.Printf("Current working directory: %s", dir)
		log.Fatalf("Error loading .env file: %v", err)
	}

	projectsDir, err = utils.GetEnv("PROJECTS_DIR")
	require.NoError(t, err)

	err = utils.InitEnvironmentVariables(projectsDir, goEnv)
	require.NoError(t, err)

	postgresUser, err := utils.GetEnv("POSTGRES_USER")
	require.NoError(t, err)

	postgresPassword, err := utils.GetEnv("POSTGRES_PASSWORD")
	require.NoError(t, err)

	postgresDb, err := utils.GetEnv("POSTGRES_DB")
	require.NoError(t, err)

	// Create a Docker network for both containers
	net, err := network.New(ctx)
	require.NoError(t, err)
	testcontainers.CleanupNetwork(t, net)

	networkName = net.Name

	// Start a eventstoredb container
	esdbReq := testcontainers.ContainerRequest{
		Image: "eventstore/eventstore:24.2.0-jammy",
		Cmd: []string{
			"--db", "/var/lib/eventstore",
			"--log", "/var/log/eventstore",
		},
		Tmpfs: map[string]string{
			"/var/lib/eventstore": "rw",
			"/var/log/eventstore": "rw",
		},
		ExposedPorts: []string{"2113/tcp", "1113/tcp"},
		Env: map[string]string{
			"EVENTSTORE_RUN_PROJECTIONS":            "All",
			"EVENTSTORE_START_STANDARD_PROJECTIONS": "true",
			"EVENTSTORE_INT_TCP_PORT":               "1113",
			"EVENTSTORE_HTTP_PORT":                  "2113",
			"EVENTSTORE_INSECURE":                   "true",
			"EVENTSTORE_ENABLE_ATOM_PUB_OVER_HTTP":  "true",
			"EVENTSTORE_EXT_IP":                     "0.0.0.0",
		},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort("2113/tcp").WithStartupTimeout(60 * time.Second),
		),
		Networks:       []string{networkName},
		NetworkAliases: map[string][]string{networkName: {"eventstoredb"}},
	}

	esdbContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: esdbReq,
		Started:          true,
	})
	testcontainers.CleanupContainer(t, esdbContainer)

	esdbStarted := false
	defer func() {
		// Capture and print the Docker logs before terminating the container
		logs, err := esdbContainer.Logs(ctx)
		require.NoError(t, err)

		if t.Failed() && !esdbStarted {
			bytes, err := io.ReadAll(logs)
			require.NoError(t, err)

			fmt.Println("Esdb logs:")
			fmt.Println(string(bytes))
		}
	}()

	require.NoError(t, err)
	esdbStarted = true

	// Start a Postgres container
	initScriptPath := filepath.Join(projectsDir, "slack-trading", "src", "backtester-api", "db", "init.sql")

	postgresReq := testcontainers.ContainerRequest{
		Image: "postgres:13",
		Tmpfs: map[string]string{
			"/var/lib/postgresql/data": "rw",
		},
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     postgresUser,
			"POSTGRES_PASSWORD": postgresPassword,
			"POSTGRES_DB":       postgresDb,
		},
		WaitingFor: wait.ForAll(
			wait.ForLog("database system is ready to accept connections"),
			wait.ForExposedPort(),
			wait.ForListeningPort("5432/tcp").WithStartupTimeout(30*time.Second),
		),
		Files: []testcontainers.ContainerFile{
			{
				HostFilePath:      initScriptPath,
				ContainerFilePath: "/docker-entrypoint-initdb.d/init.sql",
				FileMode:          0644,
			},
		},
		Networks:       []string{networkName},
		NetworkAliases: map[string][]string{networkName: {"postgres"}},
	}

	postgresContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: postgresReq,
		Started:          true,
	})
	testcontainers.CleanupContainer(t, postgresContainer)

	postgresStarted := false
	defer func() {
		// Capture and print the Docker logs before terminating the container
		logs, err := postgresContainer.Logs(ctx)
		require.NoError(t, err)

		if t.Failed() && !postgresStarted {
			bytes, err := io.ReadAll(logs)
			require.NoError(t, err)

			fmt.Println("Postgres logs:")
			fmt.Println(string(bytes))
		}
	}()

	require.NoError(t, err)
	postgresStarted = true

	return
}
