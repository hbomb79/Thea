package worker

import (
	"log"
	"sync"

	"gitlab.com/hbomb79/TPA/enum"
)

type Worker interface {
	Start() error
	Close() error
	Status() enum.WorkerStatus
	Stage() enum.PipelineStage
}

type WorkerPool struct {
	workers []Worker
	sync.Mutex
	Wg sync.WaitGroup
}

func NewWorkerPool() *WorkerPool {
	return &WorkerPool{workers: make([]Worker, 0)}
}

// StartWorkers cycles through all the workers
// currently inside the WorkerPool and creates
// a goroutine for each. The 'Start' method of
// each worker is executed concurrently.
// Additionally, this method will also add one to the
// WaitGroup inside the WorkerPool - allowing the caller
// to wait on this group until all the goroutines finish
func (pool *WorkerPool) StartWorkers() error {
	log.Printf("Starting workers in pool, amount=%v\n", len(pool.workers))
	pool.Lock()
	defer pool.Unlock()

	for _, worker := range pool.workers {
		log.Printf("Starting a worker\n")
		pool.Wg.Add(1)
		go func(pool *WorkerPool, w Worker) {
			w.Start()
			log.Printf("A worker has finished\n")
			pool.Wg.Done()
		}(pool, worker)
	}

	return nil
}

// PushWorker inserts the worker provided in to the worker pool,
// this method will first lock the mutex to ensure mutually exclusive
// access to the worker pool slice.
func (pool *WorkerPool) PushWorker(w Worker) {
	pool.Lock()
	defer pool.Unlock()

	pool.workers = append(pool.workers, w)
}

// IterWorkers will lock the worker pool's mutex, and cycle through
// all the workers associatted with this worker pool and execute
// the provided 'callback', passing the worker as a parameter.
func (pool *WorkerPool) IterWorkers(callback func(w Worker)) {
	pool.Lock()
	defer pool.Unlock()

	for _, w := range pool.workers {
		callback(w)
	}
}

// Close will cycle through all the workers inside this
// worker pool and close all the channels (notify and wait)
// While doing this, the WorkerPool's mutex is locked.
func (pool *WorkerPool) Close() {
	pool.Lock()
	defer pool.Unlock()

	for _, w := range pool.workers {
		if err := w.Close(); err != nil {
			log.Panicf("failed to close WorkerPool, a worker(%T) gave an error: %v\n", w, err.Error())
		}
	}
}
