package processor

import (
	"io/fs"
	"log"
	"path/filepath"
	"time"

	"gitlab.com/hbomb79/TPA/worker"
)

type Processor struct {
	Config     TPAConfig
	Queue      ProcessorQueue
	WorkerPool *worker.WorkerPool
}

// Instantiates a new processor by creating the
// bare struct, and loading in the configuration
func New() (proc Processor) {
	proc = Processor{Queue: make(ProcessorQueue, 0)}
	proc.Config.LoadConfig()

	proc.WorkerPool = worker.NewWorkerPool()

	return
}

// Begin will start the workers inside the WorkerPool
// responsible for the various tasks inside the program
// This includes: HTTP RESTful API (NYI), user interaction (NYI),
// import directory polling, title formatting (NYI), OMDB querying (NYI),
// and the FFMPEG formatting (NYI)
// This method will wait on the WaitGroup attached to the WorkerPool
func (p *Processor) Begin() error {
	tickInterval := time.Duration(p.Config.Format.ImportDirTickDelay * int(time.Second))
	if tickInterval <= 0 {
		log.Panic("Failed to start PollingWorker - TickInterval is non-positive (make sure 'import_polling_delay' is set in your config)")
	}

	// Normally workers would finish once all the
	// tasks in the worker pool are finished
	// However we don't want that - instead we'll
	// use notification channels to allow a worker
	// to broadcast to all other relevant workers
	// that it's done with a task. This allows
	// workers involved in the next stage of the pipeline
	// to check for new tasks if they're currently waiting
	importChan := make(worker.WorkerNotifyChan)
	//titleChan := make(chan int)
	//omdbChan := make(chan int)
	//formatChan := make(chan int)

	// Start some workers in the pool to handle
	// the import directory polling
	log.Printf("Config: %#v\n", p.Config.Concurrent)
	worker.NewPollingWorkers(p.WorkerPool, p.Config.Concurrent.Import, func(_ *worker.PollingWorker) error {
		p.PollInputSource()
		return nil
	}, time.NewTicker(tickInterval).C, importChan)

	// Wait for all the workers to finish
	// TODO: A special worker responsible for user
	// interaction might close all the Workers (pool.Close())
	// to allow the program to quit.
	p.WorkerPool.StartWorkers()
	p.WorkerPool.Wg.Wait()
	return nil
}

// PollInputSource will check the source input directory (from p.Config)
// pass along the files it finds to the p.Queue to be inserted if not present.
func (p *Processor) PollInputSource() error {
	log.Printf("Polling input source for new files")
	walkFunc := func(path string, dir fs.DirEntry, err error) error {
		if err != nil {
			log.Panicf("PollInputSource failed - %v\n", err.Error())
		}

		if !dir.IsDir() {
			v, err := dir.Info()
			if err != nil {
				log.Panicf("Failed to get FileInfo for path %v - %v\n", path, err.Error())
			}

			p.Queue.HandleFile(path, v)
		}

		return nil
	}

	filepath.WalkDir(p.Config.Format.ImportPath, walkFunc)
	return nil
}
