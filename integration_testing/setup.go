package integrationtesting

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/jiaming2012/slack-trading/src/go/playground"
	"github.com/jiaming2012/slack-trading/src/go/utils"
)

// LogConsumerCfg is a configuration for a log consumer
type LogConsumer struct{}

func (c *LogConsumer) Accept(l testcontainers.Log) {
	// if l.LogType == testcontainers.StdoutLog {
	fmt.Println(string(l.Content))
	// }
}

func createOtelCollector(ctx context.Context, t *testing.T, networkName string) testcontainers.Container {
	_, thisFile, _, _ := runtime.Caller(0)
	configPath := filepath.Join(filepath.Dir(thisFile), "otel_collector_config.yaml")
	logConsumer := &LogConsumer{}

	collectorContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "otel/opentelemetry-collector-contrib:0.96.0",
			ExposedPorts: []string{"4317/tcp", "4318/tcp", "8888/tcp"},
			// Use tmpfs for /data so the file exporter can create files
			Tmpfs: map[string]string{
				"/data": "rw",
			},
			Files: []testcontainers.ContainerFile{
				{
					HostFilePath:      configPath,
					ContainerFilePath: "/etc/otelcol-contrib/config.yaml",
					FileMode:          0644,
				},
			},
			WaitingFor: wait.ForAll(
				wait.ForLog("Everything is ready").WithStartupTimeout(30*time.Second),
			),
			Networks:       []string{networkName},
			NetworkAliases: map[string][]string{networkName: {"otel-collector"}},
			LogConsumerCfg: &testcontainers.LogConsumerConfig{Consumers: []testcontainers.LogConsumer{logConsumer}},
		},
		Started: true,
	})
	testcontainers.CleanupContainer(t, collectorContainer)
	require.NoError(t, err)

	return collectorContainer
}

func createPlaygroundServerAndClient(ctx context.Context, t *testing.T, projectDir, networkName string) playground.PlaygroundService {
	return createPlaygroundServerAndClientWithOtel(ctx, t, projectDir, networkName, false)
}

func createPlaygroundServerAndClientWithOtel(ctx context.Context, t *testing.T, projectDir, networkName string, enableOtel bool) playground.PlaygroundService {
	logConsumer := &LogConsumer{}

	env := map[string]string{
		"TRADING_PROJECT_DIR": "/app/slack-trading",
		"GO_ENV":              "test",
		"DRY_RUN":             "false",
		"POSTGRES_HOST":       "postgres",
		"POSTGRES_PORT":       "5432",
		"ANACONDA_HOME":       "/opt/conda",
		"EVENTSTOREDB_URL":    "esdb://admin:changeit@eventstoredb:2113?tls=false&keepAliveTimeout=10000&keepAliveInterval=10000",
		"LOG_LEVEL":           "debug",
	}

	if enableOtel {
		env["OTEL_EXPORTER_OTLP_ENDPOINT"] = "http://otel-collector:4318"
		env["OTEL_BSP_SCHEDULE_DELAY"] = "1000"
		env["OTEL_METRIC_EXPORT_INTERVAL"] = "5000"
	}

	appContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "grodt/app:latest-dev",
			ExposedPorts: []string{"5051/tcp"},
			Env:          env,
			WaitingFor: wait.ForAll(
				wait.ForExposedPort(),
				wait.ForListeningPort("5051/tcp").WithStartupTimeout(30*time.Second),
				wait.ForLog("Main: init complete"),
			),
			Files: []testcontainers.ContainerFile{
				{
					HostFilePath:      filepath.Join(projectDir, ".env"),
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

func setupWithOtel(t *testing.T, ctx context.Context, goEnv string) (playground.PlaygroundService, testcontainers.Container) {
	projectsDir, networkName := setupDatabases(t, ctx, goEnv)
	collector := createOtelCollector(ctx, t, networkName)
	client := createPlaygroundServerAndClientWithOtel(ctx, t, projectsDir, networkName, true)
	return client, collector
}

func setupDatabases(t *testing.T, ctx context.Context, goEnv string) (projectDir, networkName string) {
	// Derive TRADING_PROJECT_DIR from this file's location (integration_testing/ -> repo root)
	// so tests work regardless of the shell's TRADING_PROJECT_DIR value.
	_, thisFile, _, _ := runtime.Caller(0)
	projectDir = filepath.Dir(filepath.Dir(thisFile))
	os.Setenv("TRADING_PROJECT_DIR", projectDir)

	err := godotenv.Load(filepath.Join(projectDir, ".env"))
	if err != nil {
		log.Printf("Project directory: %s", projectDir)
		log.Fatalf("Error loading .env file: %v", err)
	}

	err = utils.InitEnvironmentVariables(projectDir, goEnv)
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
		Privileged:   true,
		User:         "0:0",
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
	initScriptPath := filepath.Join(projectDir, "src", "go", "backtester-api", "db", "init.sql")

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
