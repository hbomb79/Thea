package docker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/hbomb79/Thea/pkg/broker"
	"github.com/hbomb79/Thea/pkg/logger"
)

var dockerLogger = logger.Get("Docker")

/**
 * The docker package provides utilities for Thea with regards to creating, fetching and spawning docker images/containers
 * locally. This is used to spawn services such as Theas PostgreSQL database, or the NPM front end.
 */

const DockerNetworkName = "thea_network"

type DockerManager interface {
	SpawnContainer(container DockerContainer) error
	Shutdown(timeout time.Duration)
	CloseContainer(name string, timeout time.Duration)
	WaitForContainer(container DockerContainer, statuses ...ContainerStatus) (ContainerStatus, error)
}

type dockerContainerStatus struct {
	containerLabel string
	status         ContainerStatus
}

type docker struct {
	containers map[string]DockerContainer
	cli        *client.Client
	ctx        context.Context
	ctxCancel  context.CancelFunc
	wg         *sync.WaitGroup
	broker     *broker.Broker[*dockerContainerStatus]
}

func NewDockerManager() DockerManager {
	// TODO Proper context handling!
	ctx, ctxCancel := context.WithCancel(context.TODO())
	c, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}

	_, err = c.NetworkCreate(ctx, DockerNetworkName, types.NetworkCreate{
		CheckDuplicate: true,
		Driver:         "bridge",
	})
	if err != nil {
		dockerLogger.Emit(
			logger.WARNING,
			"Failed to create docker container network: %v. This is usually safe to ignore if the error is due to the network already existing\n",
			err,
		)
	}

	broker := broker.NewBroker[*dockerContainerStatus]()
	go broker.Start()
	return &docker{
		containers: make(map[string]DockerContainer),
		ctx:        ctx,
		ctxCancel:  ctxCancel,
		cli:        c,
		wg:         &sync.WaitGroup{},
		broker:     broker,
	}
}

func (docker *docker) SpawnContainer(container DockerContainer) error {
	if _, ok := docker.containers[container.Label()]; ok {
		return fmt.Errorf("cannot spawn container %s as label is already in use", container)
	} else {
		docker.containers[container.Label()] = container
	}

	docker.wg.Add(1)
	if err := container.Start(docker.ctx, docker.cli); err != nil {
		container.Close(docker.ctx, docker.cli, time.Second*10)
		docker.wg.Done()
		return err
	}

	if err := docker.cli.NetworkConnect(docker.ctx, DockerNetworkName, container.ID(), nil); err != nil {
		dockerLogger.Emit(logger.ERROR, "Failed to connect container %s to network: %v\n", container, err)
	}

	go docker.monitorContainer(container, docker.wg)

	dockerLogger.Emit(logger.INFO, "Waiting for container %s to come UP\n", container)
	if _, err := docker.WaitForContainer(container, UP); err != nil {
		dockerLogger.Emit(logger.ERROR, "Container %s failed to come online: %v\n", container, err)
		return err
	}

	dockerLogger.Emit(logger.SUCCESS, "Container %s is UP!\n", container)
	return nil
}

func (docker *docker) Shutdown(timeout time.Duration) {
	for _, c := range docker.containers {
		docker.closeContainer(c, timeout)
	}

	docker.wg.Wait()
	if err := docker.cli.NetworkRemove(docker.ctx, DockerNetworkName); err != nil {
		dockerLogger.Warnf("Failed to remove docker network: %s\n", err)
	}
}

func (docker *docker) CloseContainer(name string, timeout time.Duration) {
	container, ok := docker.containers[name]
	if !ok {
		return
	}

	docker.closeContainer(container, timeout)
}

func (docker *docker) WaitForContainer(container DockerContainer, statuses ...ContainerStatus) (ContainerStatus, error) {
	ch := docker.broker.Subscribe()
	defer docker.broker.Unsubscribe(ch)

	// If container is DEAD we won't ever see a status change
	if container.Status() == DEAD {
		return DEAD, fmt.Errorf("cannot wait on DEAD container %s", container)
	}

	// If container is already the state we want
	for _, s := range statuses {
		if container.Status() == s {
			return s, nil
		}
	}

	// Wait for the container to have one of the statuses we want
	for update := range ch {
		if update.containerLabel == container.Label() {
			for _, stat := range statuses {
				if stat == update.status {
					return stat, nil
				}
			}
		}
	}

	return DEAD, fmt.Errorf("wait on container %s aborted as container has closed", container)
}

func (docker *docker) closeContainer(container DockerContainer, timeout time.Duration) {
	dockerLogger.Emit(logger.STOP, "Closing container %s...\n", container)
	container.Close(docker.ctx, docker.cli, timeout)

	dockerLogger.Emit(logger.STOP, "Waiting for container %s to change state to DEAD...\n", container)
	if _, err := docker.WaitForContainer(container, DEAD); err != nil {
		dockerLogger.Warnf("Failed while waiting for container %s to change state to DEAD: %s\n", container, err)
	}
}

func (docker *docker) monitorContainer(container DockerContainer, wg *sync.WaitGroup) {
	defer func() {
		dockerLogger.Emit(logger.INFO, "Container %s - Status management DETACHED\n", container)
		wg.Done()
	}()

	for {
		select {
		case stat, ok := <-container.StatusChannel():
			if !ok {
				return
			}
			dockerLogger.Emit(logger.INFO, "Container %s - Status change: %s\n", container, stat)

			docker.broker.Publish(&dockerContainerStatus{containerLabel: container.Label(), status: stat})
		case stat, ok := <-container.MessageChannel():
			if !ok {
				return
			}
			dockerLogger.Emit(logger.DEBUG, "%s: %s\n", container, stat)
		}
	}
}
