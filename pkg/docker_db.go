package pkg

import (
	"fmt"
	"os"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/go-connections/nat"
)

// DatabaseConfig is a subset of the configuration focusing solely
// on database connection items
type DatabaseConfig struct {
	User     string `yaml:"username" env:"DB_USERNAME" env-required:"true"`
	Password string `yaml:"password" env:"DB_PASSWORD" env-required:"true"`
	Name     string `yaml:"name" env:"DB_NAME" env-default:"TPA_DB"`
	Host     string `yaml:"host" env:"DB_HOST" env-default:"0.0.0.0"`
	Port     string `yaml:"port" env:"DB_PORT" env-default:"5432"`
}

func InitialiseDockerDatabase(config DatabaseConfig, errChannel chan error) (DockerContainer, error) {
	// Setup container cofiguration
	dbDataPath := "/home/haz/tpa_db_data"
	os.MkdirAll(dbDataPath, os.ModeDir)
	containerConfig := &container.Config{
		Image: "postgres:14.1-alpine",
		Env: []string{
			fmt.Sprintf("POSTGRES_PASSWORD=%s", config.Password),
			fmt.Sprintf("POSTGRES_USER=%s", config.User),
			fmt.Sprintf("POSTGRES_DB=%s", config.Name),
			fmt.Sprintf("DATABASE_HOST=%s", config.Host),
		},
		ExposedPorts: nat.PortSet{
			"5432": struct{}{},
		},
	}
	hostConfig := &container.HostConfig{
		PortBindings: nat.PortMap{
			nat.Port(config.Port): []nat.PortBinding{{
				HostIP:   config.Host,
				HostPort: config.Port,
			}},
		},
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: dbDataPath,
				Target: "/var/lib/postgresql/data",
			},
		},
	}

	// Spawn docker container for postgres
	db := NewDockerContainer("db", "postgres:14.1-alpine", containerConfig, hostConfig)
	if err := Docker.SpawnContainer(db); err != nil {
		return nil, err
	}

	// Watch for container crash (teardown)
	go func() {
		st, err := Docker.WaitForContainer(db, CRASHED)
		if st != CRASHED || err != nil {
			return
		}

		errChannel <- fmt.Errorf("container %s has crashed", db)
	}()

	return db, nil
}
