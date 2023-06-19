package download

import "context"

type downloadService struct{}

func New() *downloadService {
	return &downloadService{}
}

func (service *downloadService) Start(ctx context.Context) {}
