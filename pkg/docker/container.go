package docker

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	dCont "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/hbomb79/Thea/pkg/logger"
)

type ContainerStatus int

const (
	// Container struct instance has just been created.
	INIT ContainerStatus = iota

	// Container image has been pulled to local docker daemon, but the container has not yet been created.
	PULLED

	// Container has been created from a previously PULLED image.
	CREATED

	// Container is UP and working normally.
	UP

	// Container has CRASHED.
	CRASHED

	// Container is being closed intentionally, next status should always be DOWN.
	CLOSING

	// Container is DOWN (intentionally closed).
	DOWN

	// Container has been removed.
	DEAD
)

type ContainerEvent struct {
	Status         string `json:"status"`
	Error          string `json:"error"`
	Progress       string `json:"progress"`
	ProgressDetail struct {
		Current int `json:"current"`
		Total   int `json:"total"`
	} `json:"progressDetail"` //nolint:tagliatelle
}

func (e ContainerStatus) String() string {
	return []string{"INIT", "PULLED", "CREATED", "UP", "CRASHED", "CLOSING", "DOWN", "DEAD"}[e]
}

type DockerContainer interface {
	// Start will pull the required Docker image and attempt to create and start
	// a container via the Docker SDK. An error will be returned from this method if
	// this process fails, however monitoring of this container occurs asynchronously
	// so no error will be returned if the container crashes after successfully starting.
	Start(ctx context.Context, client client.APIClient) error

	// Close shuts down this container by killing the running container (if running), and
	// removing the container from the docker daemon via the Docker SDK. If closing or removing
	// the container fails, this method will return an error.
	Close(ctx context.Context, client client.APIClient, timeout time.Duration) error

	// MessageChannel returns the channel used by a running container to broadcast new
	// messages from the stdout/stderr of the container. A DEAD container will have a closed
	// message channel.
	MessageChannel() chan []byte

	// StatusChannel returns the channel used by a container to broadcast it's status (see ContainerStatus)
	// A channel that has broadcast a DEAD state will soon close this channel.
	StatusChannel() chan ContainerStatus

	// Label returns the label of this container
	Label() string

	// ID returns the container ID of this container.
	ID() string

	// Status returns the current status of this container. To receive updates of this status in real-time, use
	// the StatusChannel()
	Status() ContainerStatus
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

// NewDockerContainer creates a new DockerContainer instance. This instance can later be started manually, or via a Docker
// container management system (see pkg.Docker).
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

func (container *dockerContainer) Start(ctx context.Context, cli client.APIClient) error {
	if container.status != INIT {
		return fmt.Errorf("cannot start container %s based on image %v as status is invalid", container, container.imageID)
	}

	out, err := cli.ImagePull(ctx, container.imageID, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image %v for container %s: %w", container.imageID, container, err)
	}
	defer out.Close()

	eventStream := json.NewDecoder(out)
	var event *ContainerEvent
	for {
		if err := eventStream.Decode(&event); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			panic(err)
		}

		container.parseContainerEvent(event)
	}

	container.setStatus(PULLED)

	resp, err := cli.ContainerCreate(ctx, container.containerConf, container.containerHostConf, nil, nil, container.label)
	if err != nil {
		return fmt.Errorf("failed to create container for %s: %w", container, err)
	}
	container.containerID = resp.ID
	container.setStatus(CREATED)

	if err := cli.ContainerStart(ctx, resp.ID, dCont.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container for %s: %w", container, err)
	}
	container.setStatus(UP)

	go container.monitorContainer(ctx, cli)
	return nil
}

func (container *dockerContainer) Close(ctx context.Context, cli client.APIClient, timeout time.Duration) error {
	if container.status == DEAD {
		return nil
	}

	if container.canStop() {
		container.setStatus(CLOSING)
		timeoutSeconds := int(timeout.Seconds())
		if err := cli.ContainerStop(ctx, container.containerID, dCont.StopOptions{Timeout: &timeoutSeconds}); err != nil {
			return fmt.Errorf("failed to stop container %s: %w", container, err)
		}

		container.setStatus(DOWN)
	}

	if container.canRemove() {
		if err := cli.ContainerRemove(ctx, container.containerID, dCont.RemoveOptions{}); err != nil {
			return fmt.Errorf("failed to remove container %s: %w", container, err)
		}
	}
	container.setStatus(DEAD)

	close(container.statusChannel)
	close(container.messageChannel)

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

func (container *dockerContainer) parseContainerEvent(ev *ContainerEvent) {
	switch {
	case ev.Error != "":
		dockerLogger.Emit(logger.ERROR, "\n%s: %s\n", container, ev.Error)
	case ev.Progress != "":
		dockerLogger.Emit(logger.DEBUG, "%s: %s\n", container, ev.Progress)
	case ev.Status != "":
		dockerLogger.Emit(logger.DEBUG, "%s: %s\n", container, ev.Status)
	default:
		dockerLogger.Emit(logger.WARNING, "Container %s emitted unknown event %v\n", container, ev)
	}
}

func (container *dockerContainer) monitorContainer(ctx context.Context, cli client.APIClient) {
	reader, err := cli.ContainerLogs(ctx, container.containerID, dCont.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: false,
		Details:    false,
	})
	if err != nil {
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
