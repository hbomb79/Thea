package helpers

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	mutex   = sync.Mutex{}
	portInc = 42067
)

func getNextPort() int {
	mutex.Lock()
	defer mutex.Unlock()

	portInc++
	return portInc
}

func SpawnTheaManualCleanup(databaseName string) *TestService {
	port := getNextPort()
	fmt.Printf("Spawning Thea on port %d\n", port)
	dbManager.SeedNewDatabase(databaseName)
	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:       "../../",
			KeepImage:     true,
			PrintBuildLog: true,
		},
		ExposedPorts: []string{fmt.Sprintf("%d/tcp", port)},
		WaitingFor:   wait.ForLog("Thea services spawned!").WithStartupTimeout(time.Second * 4),
		Env: map[string]string{
			"API_HOST_ADDR": fmt.Sprintf("0.0.0.0:%d", port),
			"DB_NAME":       databaseName,
		},
		NetworkMode: "host",
		Cmd:         []string{"-config", "/config.toml"},
	}

	theaC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: req, Started: true})
	if err != nil {
		log.Fatalf("Could not start Thea: %s", err)
	}

	return &TestService{
		Port:         port,
		DatabaseName: databaseName,
		cleanup: func() {
			fmt.Printf("Stopping Thea...")
			timeout := 5 * time.Second
			if err := theaC.Stop(ctx, &timeout); err != nil {
				fmt.Printf("Could not stop Thea: %s", err)
			}
		},
	}
}

func SpawnPostgres() func() {
	dbName := "THEA_DB"
	dbUser := "postgres"
	dbPassword := "postgres"

	postgresC, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("docker.io/postgres:14.1-alpine"),
		postgres.WithDatabase(dbName),
		postgres.WithUsername(dbUser),
		postgres.WithPassword(dbPassword),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Second)),
		testcontainers.WithHostConfigModifier(func(hostConfig *container.HostConfig) { hostConfig.NetworkMode = "host" }),
	)
	if err != nil {
		log.Fatalf("failed to start container: %s", err)
	}

	// Clean up the container
	return func() {
		fmt.Printf("Stopping postgres...")
		timeout := 5 * time.Second
		if err := postgresC.Stop(ctx, &timeout); err != nil {
			fmt.Printf("Could not stop Postgres: %s", err)
		}
	}
}
