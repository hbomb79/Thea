package activity

import "context"

/*
 * Activity service is responsible for listening for relevant events,
 * and emitting update messages over the websocket.
 */

type activityService struct{}

func New() *activityService {
	return &activityService{}
}

func (service *activityService) Start(ctx context.Context) {}
