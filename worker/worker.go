package worker

type WorkerStatus interface {
	Finished() bool
	PercentDone() float64
}

type Worker interface {
	Status() WorkerStatus
	Start(inboundChannel chan interface{}, outboundWorker chan interface{}) error
}

func StartWorkers(amount int, worker Worker, inboundChannel chan interface{}, outboundChannel chan interface{}) error {
	return nil
}
