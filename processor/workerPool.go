package processor

import (
	"fmt"
	"log"
	"sync"
)

// WorkerPool struct embeds the sync.Mutex struct, and
// also contains a sync.WaitGroup as 'wg'. The WaitGroup is
// automatically controlled by the WorkerPool. The 'workers'
// field is a slice that contains all the workers
// attached to this WorkerPool
type WorkerPool struct {
	workers []*Worker
	Wg      sync.WaitGroup
}

// NewWorkerPool creates a new WorkerPool struct
// and initialises the 'workers' slice
func NewWorkerPool() *WorkerPool {
	return &WorkerPool{workers: make([]*Worker, 0)}
}

// NewWorkers creates worker instances (n=amount)
// and labels them with the label provided (appended with
// the worker ID derived from the iterator used to create
// the workers).
func (pool *WorkerPool) NewWorkers(amount int, workerLabel string, workerTask WorkerTask, wakeupChannel chan int, pipelineStage PipelineStage) {
	log.Printf("Creating %v workers labelled '%v'\n", amount, workerLabel)
	for i := 0; i < amount; i++ {
		pool.PushWorker(NewWorker(fmt.Sprintf("%v:%v", workerLabel, i), workerTask, wakeupChannel, pipelineStage))
	}
}

// StartWorkers cycles through all the workers
// currently inside the WorkerPool and creates
// a goroutine for each. The 'Start' method of
// each worker is executed concurrently.
func (pool *WorkerPool) StartWorkers() {
	for _, worker := range pool.workers {
		pool.Wg.Add(1)
		go func(pool *WorkerPool, w *Worker) {
			w.Start()
			pool.Wg.Done()
		}(pool, worker)
	}
}

// PushWorker inserts the worker provided in to the worker pool,
// this method will first lock the mutex to ensure mutually exclusive
// access to the worker pool slice.
func (pool *WorkerPool) PushWorker(w *Worker) {
	pool.workers = append(pool.workers, w)
}

// WakeupWorkers will search for workers in the pool
// that are responsible for the stage of the pipeline
// provided (stage) and will send on their WakeupChannel
// to wake up sleeping workers
func (pool *WorkerPool) WakeupWorkers(stage PipelineStage) {
	for _, w := range pool.workers {
		if w.Stage() == stage && w.Status() == Idle {
			w.WakeupChan() <- 1
		}
	}
}

// CloseWorkers will cycle through all the workers inside this
// worker pool and close all the channels (notify and wait)
// While doing this, the WorkerPool's mutex is locked.
func (pool *WorkerPool) CloseWorkers() {
	for _, w := range pool.workers {
		if err := w.Close(); err != nil {
			log.Panicf("failed to close WorkerPool, a worker(%v) gave an error: %v\n", w.label, err.Error())
		}
	}
}
