package worker

import (
	"fmt"
	"sync"
)

// WorkerPool struct embeds the sync.Mutex struct, and
// also contains a sync.WaitGroup as 'wg'. The WaitGroup is
// automatically controlled by the WorkerPool. The 'workers'
// field is a slice that contains all the workers
// attached to this WorkerPool
type WorkerPool struct {
	workers []Worker
	Wg      sync.WaitGroup
}

// NewWorkerPool creates a new WorkerPool struct
// and initialises the 'workers' slice
func NewWorkerPool() *WorkerPool {
	return &WorkerPool{workers: make([]Worker, 0)}
}

// StartWorkers cycles through all the workers
// currently inside the WorkerPool and creates
// a goroutine for each. The 'Start' method of
// each worker is executed concurrently.
func (pool *WorkerPool) StartWorkers(pWg *sync.WaitGroup) {
	defer pWg.Done()
	for _, worker := range pool.workers {
		pool.Wg.Add(1)
		go func(pool *WorkerPool, w Worker) {
			defer pool.Wg.Done()
			w.Start()
		}(pool, worker)
	}

	pool.Wg.Wait()
}

// PushWorker inserts the worker provided in to the worker pool,
// this method will first lock the mutex to ensure mutually exclusive
// access to the worker pool slice.
func (pool *WorkerPool) PushWorker(workers ...Worker) {
	pool.workers = append(pool.workers, workers...)
}

// WakeupWorkers will search for sleeping workers in the pool
// and will send on their WakeupChannel to wake up sleeping workers
func (pool *WorkerPool) WakeupWorkers() {
	for _, w := range pool.workers {
		if w.Status() == Sleeping {
			select {
			case w.WakeupChan() <- 1:
			default:
			}
		}
	}
}

// CloseWorkers will cycle through all the workers inside this
// worker pool and close all the channels (notify and wait)
// While doing this, the WorkerPool's mutex is locked.
func (pool *WorkerPool) CloseWorkers() {
	for _, w := range pool.workers {
		fmt.Printf("[WorkerPool] (X) Closing worker [%v]...\n", w.Label())
		w.Close()
	}
}
