package database

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/go-connections/nat"
	"github.com/hbomb79/Thea/pkg/docker"
)

// DatabaseConfig is a subset of the configuration focusing solely
// on database connection items
type DatabaseConfig struct {
	User     string `yaml:"username" env:"DB_USERNAME" env-required:"true"`
	Password string `yaml:"password" env:"DB_PASSWORD" env-required:"true"`
	Name     string `yaml:"name" env:"DB_NAME" env-default:"THEA_DB"`
	Host     string `yaml:"host" env:"DB_HOST" env-default:"0.0.0.0"`
	Port     string `yaml:"port" env:"DB_PORT" env-default:"5432"`
}

func InitialiseDockerDatabase(dockerManager docker.DockerManager, config DatabaseConfig, errChannel chan error) (docker.DockerContainer, error) {
	// Setup container cofiguration
	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic(fmt.Sprintf("Cannot initialize docker db volume mount as cannot find user home dir: %s", err.Error()))
	}

	dbDataPath := filepath.Join(homeDir, "thea_db.dat")
	if err := os.MkdirAll(dbDataPath, os.ModeDir); err != nil {
		return nil, err
	}

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
	db := docker.NewDockerContainer("db", "postgres:14.1-alpine", containerConfig, hostConfig)
	if err := dockerManager.SpawnContainer(db); err != nil {
		return nil, err
	}

	// Watch for container crash (teardown)
	go func() {
		st, err := dockerManager.WaitForContainer(db, docker.CRASHED)
		if st != docker.CRASHED || err != nil {
			return
		}

		errChannel <- fmt.Errorf("container %s has crashed", db)
	}()

	return db, nil
}

func InitialiseDockerPgAdmin(dockerManager docker.DockerManager, errChannel chan error) (docker.DockerContainer, error) {
	// Setup container cofiguration
	containerConfig := &container.Config{
		Image: "dpage/pgadmin4",
		Env: []string{
			"PGADMIN_DEFAULT_EMAIL=admin@admin.com",
			"PGADMIN_DEFAULT_PASSWORD=root",
		},
		ExposedPorts: nat.PortSet{
			"80": struct{}{},
		},
	}
	hostConfig := &container.HostConfig{
		PortBindings: nat.PortMap{
			"80": []nat.PortBinding{{
				HostIP:   "0.0.0.0",
				HostPort: "5050",
			}},
		},
	}

	// Spawn docker container for postgres
	db := docker.NewDockerContainer("pgAdmin", "dpage/pgadmin4", containerConfig, hostConfig)
	if err := dockerManager.SpawnContainer(db); err != nil {
		return nil, err
	}

	// Watch for container crash (teardown)
	go func() {
		st, err := dockerManager.WaitForContainer(db)
		if st != docker.CRASHED || err != nil {
			return
		}

		errChannel <- fmt.Errorf("container %s has crashed", db)
	}()

	return db, nil
}
