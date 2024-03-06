package helpers

import (
	"fmt"
	"maps"
	"sync"
	"testing"
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

// spawnThea will spawn a Thea service instance in it's own Docker container. The
// container will have it's environment variables and volumes
// set as per the request provided, and the function will only return once the container
// has successfully started. Each service will receive a unique port number.
func spawnThea(t *testing.T, req TheaServiceRequest) *TestService {
	if req.databaseName == "" {
		t.Fatalf("cannot satisfy Thea container request %#v as no databaseName is specified. Implicit fallback to master DB is disallowed, explicit database must be provided", req)
		return nil
	}

	port := getNextPort()
	t.Logf("Spawning Thea on port %d for request %s\n", port, req)
	databaseName := req.databaseName

	envVars := make(map[string]string)
	maps.Copy(envVars, req.environmentVariables)
	envVars["API_HOST_ADDR"] = fmt.Sprintf("0.0.0.0:%d", port)
	envVars["DB_NAME"] = databaseName

	dockerReq := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{Context: "../../", KeepImage: true, PrintBuildLog: true},
		ExposedPorts:   []string{fmt.Sprintf("%d/tcp", port)},
		WaitingFor:     wait.ForLog("Thea services spawned!").WithStartupTimeout(time.Second * 4),
		Env:            envVars,
		NetworkMode:    "host",
		Cmd:            []string{"-config", "/config.toml"},
		LogConsumerCfg: &testcontainers.LogConsumerConfig{
			Opts:      []testcontainers.LogProductionOption{testcontainers.WithLogProductionTimeout(10 * time.Second)},
			Consumers: []testcontainers.LogConsumer{&LogConsumer{t, fmt.Sprintf("db=%s port=%d", databaseName, port)}},
		},
	}

	if req.ingestDirectory != "" {
		volumeName := fmt.Sprintf("thea-%d-volume", port)
		dockerReq.Mounts = testcontainers.ContainerMounts{
			{
				Source: testcontainers.GenericVolumeMountSource{Name: volumeName},
				Target: "/ingests",
			},
		}
	}

	theaC, err := testcontainers.GenericContainer(ctx,
		testcontainers.GenericContainerRequest{
			ContainerRequest: dockerReq,
			Started:          true,
			Logger:           testcontainers.TestLogger(t),
		})
	if err != nil {
		t.Fatalf("Could not start Thea: %s", err)
	}

	return &TestService{
		Port:         port,
		DatabaseName: databaseName,
		container:    theaC,
		cleanup: func(t *testing.T) {
			t.Log("Stopping Thea...")
			timeout := 5 * time.Second
			if err := theaC.Stop(ctx, &timeout); err != nil {
				t.Logf("Could not stop Thea: %s", err)
			}
		},
	}
}

func spawnPostgres(t *testing.T) testcontainers.Container {
	postgresC, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("docker.io/postgres:14.1-alpine"),
		postgres.WithDatabase(MasterDBName),
		postgres.WithUsername(User),
		postgres.WithPassword(Password),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Second)),
		testcontainers.WithHostConfigModifier(func(hostConfig *container.HostConfig) { hostConfig.NetworkMode = "host" }),
	)
	if err != nil {
		t.Fatalf("failed to start container: %s", err)
		return nil
	}

	return postgresC
}

type LogConsumer struct {
	t              *testing.T
	theaIdentifier string
}

func (lc *LogConsumer) Accept(l testcontainers.Log) {
	if l.LogType == testcontainers.StdoutLog {
		lc.t.Logf("[Thea | %s] -> %s", lc.theaIdentifier, string(l.Content))
	} else {
		lc.t.Logf("[Thea | %s] WARNING -> %s", lc.theaIdentifier, string(l.Content))
	}
}
