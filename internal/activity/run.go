package activity

import "context"

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

func (service *ActivityService) Run(ctx context.Context) {
	// Listen to events we need to forward over the activity bus
}

func (service *ActivityService) RegisterEventCoordinator(ev EventCoordinator) {
	service.eventBus = ev
}
