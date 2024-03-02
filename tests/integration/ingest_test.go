package integration_test

import (
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// SpawnThea instantiates an ephemeral Thea service (which connects
// to an existing database, as per it's configuration).
// This has advantages over using an externally available
// service as the test can setup the configuration
// to ensure the tests are deterministic, while still testing
// all layers of the application.
func SpawnThea(t *testing.T) {
	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:       "../../",
			KeepImage:     true,
			PrintBuildLog: true,
		},
		ExposedPorts: []string{"42069/tcp"},
		WaitingFor:   wait.ForLog("Thea services spawned!").WithStartupTimeout(time.Second * 4),
		Env: map[string]string{
			"API_HOST_ADDR": "0.0.0.0:42069",
		},
		NetworkMode: "host",
		Cmd:         []string{"-config", "/config.toml"},
	}

	theaC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: req, Started: true})
	if err != nil {
		t.Errorf("Could not start Thea: %s", err)
	}

	t.Cleanup(func() {
		t.Log("Stopping Thea...")
		timeout := 5 * time.Second
		if err := theaC.Stop(ctx, &timeout); err != nil {
			fmt.Printf("Could not stop Thea: %s", err)
		}
	})
}

func spawnPostgres() func() {
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
		log.Printf("Stopping postgres...")
		timeout := 5 * time.Second
		if err := postgresC.Stop(ctx, &timeout); err != nil {
			fmt.Printf("Could not stop Thea: %s", err)
		}
	}
}
