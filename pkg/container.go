package pkg

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/docker/docker/api/types"
	dCont "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

type ContainerStatus int

const (
	// Container struct instance has just been created
	INIT ContainerStatus = iota

	// Container image has been pulled to local docker daemon, but the container has not yet been created
	PULLED

	// Container has been created from a previously PULLED image
	CREATED

	// Container is UP and working normally
	UP

	// Container has CRASHED
	CRASHED

	// Container is being closed intentionally, next status should always be DOWN
	CLOSING

	// Container is DOWN (intentionally closed)
	DOWN

	// Container has been removed
	DEAD
)

func (e ContainerStatus) String() string {
	return []string{"INIT", "PULLED", "CREATED", "UP", "CRASHED", "CLOSING", "DOWN", "DEAD"}[e]
}

type DockerContainer interface {
	Start(context.Context, client.APIClient) error
	Close(context.Context, client.APIClient, time.Duration) error
	MessageChannel() chan []byte
	StatusChannel() chan ContainerStatus
	Label() string
	ID() string
	Status() ContainerStatus
	monitorContainer(ctx context.Context, cli client.APIClient)
}

type dockerContainer struct {
	statusChannel     chan ContainerStatus
	messageChannel    chan []byte
	label             string
	imageID           string
	containerID       string
	status            ContainerStatus
	containerConf     *dCont.Config
	containerHostConf *dCont.HostConfig
}

func NewDockerContainer(label string, image string, conf *dCont.Config, hostConf *dCont.HostConfig) DockerContainer {
	return &dockerContainer{
		statusChannel:     make(chan ContainerStatus, 5),
		messageChannel:    make(chan []byte, 5),
		imageID:           image,
		containerConf:     conf,
		containerHostConf: hostConf,
		status:            INIT,
		label:             label,
	}
}

func (c *dockerContainer) Start(ctx context.Context, cli client.APIClient) error {
	if c.status != INIT {
		return fmt.Errorf("cannot start container %s based on image %v as status is invalid", c, c.imageID)
	}

	out, err := cli.ImagePull(ctx, c.imageID, types.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image %v for container %s: %v", c.imageID, c, err.Error())
	}
	defer out.Close()
	io.Copy(os.Stdout, out)
	c.setStatus(PULLED)

	resp, err := cli.ContainerCreate(ctx, c.containerConf, c.containerHostConf, nil, nil, "")
	if err != nil {
		return fmt.Errorf("failed to create container for %s: %v", c, err.Error())
	}
	c.containerID = resp.ID
	c.setStatus(CREATED)

	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("failed to start container for %s: %v", c, err.Error())
	}
	c.setStatus(UP)

	go c.monitorContainer(ctx, cli)
	return nil
}

func (c *dockerContainer) Close(ctx context.Context, cli client.APIClient, timeout time.Duration) error {
	if c.status == DEAD {
		return nil
	}

	if c.canStop() {
		c.setStatus(CLOSING)
		if err := cli.ContainerStop(ctx, c.containerID, &timeout); err != nil {
			return fmt.Errorf("failed to stop container %s: %v", c, err.Error())
		}

		c.setStatus(DOWN)
	} else {
		fmt.Printf("Container already stopped/never started, skipping.\n")
	}

	if c.canRemove() {
		if err := cli.ContainerRemove(ctx, c.containerID, types.ContainerRemoveOptions{}); err != nil {
			return fmt.Errorf("failed to remove container %s: %v", c, err.Error())
		}
	} else {
		fmt.Printf("Container already removed/never created, skipping.\n")
	}
	c.setStatus(DEAD)

	close(c.statusChannel)
	close(c.messageChannel)

	return nil
}

func (container *dockerContainer) MessageChannel() chan []byte {
	return container.messageChannel
}

func (container *dockerContainer) StatusChannel() chan ContainerStatus {
	return container.statusChannel
}

func (container *dockerContainer) ID() string {
	return container.containerID
}

func (container *dockerContainer) Label() string {
	return container.label
}

func (container *dockerContainer) Status() ContainerStatus {
	return container.status
}

func (container *dockerContainer) String() string {
	if container.containerID == "" {
		return fmt.Sprintf("%v[...]", container.label)
	}

	return fmt.Sprintf("%v[%v]", container.label, container.containerID[:10])
}

func (container *dockerContainer) canStop() bool {
	return container.status == CLOSING || container.status == CREATED || container.status == UP || container.status == CRASHED
}

func (container *dockerContainer) canRemove() bool {
	return container.canStop() || container.status == DOWN || container.status == CRASHED
}

func (container *dockerContainer) setStatus(stat ContainerStatus) {
	if container.status == DEAD {
		return
	}

	container.status = stat
	container.statusChannel <- container.status
}

func (container *dockerContainer) monitorContainer(ctx context.Context, cli client.APIClient) {
	reader, err := cli.ContainerLogs(ctx, container.containerID, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: false,
		Details:    false,
	})
	if err != nil {
		fmt.Printf("[Docker] (!) Unable to open container %s for log reading - %v\n", container, err.Error())
		container.setStatus(CRASHED)
		return
	}
	defer reader.Close()

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		if container.status != UP {
			break
		}

		container.messageChannel <- scanner.Bytes()
	}

	if container.status != CLOSING {
		container.setStatus(CRASHED)
	}
}
