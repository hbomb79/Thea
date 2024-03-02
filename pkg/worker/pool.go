package worker

import (
	"errors"
	"sync"
	"time"
)

// WorkerPool struct embeds the sync.Mutex struct, and
// also contains a sync.WaitGroup as 'wg'. The WaitGroup is
// automatically controlled by the WorkerPool. The 'workers'
// field is a slice that contains all the workers
// attached to this WorkerPool.
type WorkerPool struct {
	workers []Worker
	Wg      sync.WaitGroup
	started bool
}

// NewWorkerPool creates a new WorkerPool struct
// and initialises the 'workers' slice.
func NewWorkerPool() *WorkerPool {
	return &WorkerPool{workers: make([]Worker, 0)}
}

// Start cycles through all the workers
// currently inside the WorkerPool and creates
// a goroutine for each. The 'Start' method of
// each worker is executed concurrently.
//
// Start does NOT block, however consumers
// can wait on the WaitGroup in the pool if they
// wish.
func (pool *WorkerPool) Start() error {
	if pool.started {
		return errors.New("cannot start an already started worker pool")
	}

	pool.started = true
	for _, worker := range pool.workers {
		pool.Wg.Add(1)
		go func(wg *sync.WaitGroup, w Worker) {
			defer wg.Done()
			w.Start()
		}(&pool.Wg, worker)
	}

	return nil
}

// PushWorker inserts the worker provided in to the worker pool,
// this method will first lock the mutex to ensure mutually exclusive
// access to the worker pool slice.
func (pool *WorkerPool) PushWorker(workers ...Worker) error {
	if pool.started {
		return errors.New("cannot push worker to already started worker pool")
	}

	pool.workers = append(pool.workers, workers...)
	return nil
}

// WakeupWorkers will search for sleeping workers in the pool
// and will send on their WakeupChannel to wake up sleeping workers.
func (pool *WorkerPool) WakeupWorkers() error {
	if !pool.started {
		return errors.New("cannot wakeup workers on worker pool that is not started")
	}

	for _, w := range pool.workers {
		if w.Status() == SLEEPING {
			select {
			case w.WakeupChan() <- 1:
			default:
			}
		}
	}

	return nil
}

// Close will cycle through all the workers inside this
// worker pool and close their wakeup channels.
func (pool *WorkerPool) Close() {
	if !pool.started {
		return
	}

	for _, w := range pool.workers {
		w.Close()
	}
	pool.Wg.Wait()
	pool.started = false
}
