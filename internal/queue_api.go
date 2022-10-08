package internal

// QueueService is responsible for exposing methods for reading or mutating
// the state of the TPA queue.
type QueueService interface {
	Index() error
	GetDetails() error
	Reorder() error
	Promote() error
}

type queueService struct {
	tpa TPA
}

func (queueApi *queueService) Index() error {
	return nil
}

func (queueApi *queueService) GetDetails() error {
	return nil
}

func (queueApi *queueService) Reorder() error {
	return nil
}

func (queueApi *queueService) Promote() error {
	return nil
}

func NewQueueApi(tpa TPA) QueueService {
	return &queueService{
		tpa: tpa,
	}
}
