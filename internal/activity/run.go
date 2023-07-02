package activity

import (
	"context"
	"errors"
)

/*
 * Activity service is responsible for listening for relevant events,
 * and emitting update messages over the websocket.
 */

type (
	ActivityService struct {
		eventBus EventCoordinator
	}
)

func New() (*ActivityService, error) {
	return &ActivityService{}, nil
}

func (service *ActivityService) Run(ctx context.Context) error {
	// Listen to events we need to forward over the activity bus
	return errors.New("blah")
}

func (service *ActivityService) RegisterEventCoordinator(ev EventCoordinator) {
	service.eventBus = ev
}
