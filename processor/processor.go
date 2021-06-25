package processor

import (
	"io/fs"
	"log"
	"path/filepath"
	"time"

	"gitlab.com/hbomb79/TPA/enum"
	"gitlab.com/hbomb79/TPA/worker"
)

type Processor struct {
	Config     TPAConfig
	Queue      ProcessorQueue
	WorkerPool *worker.WorkerPool
}

// Instantiates a new processor by creating the
// bare struct, and loading in the configuration
func New() *Processor {
	proc := &Processor{
		Queue: ProcessorQueue{
			Items: make([]QueueItem, 0),
		},
	}

	proc.Config.LoadConfig()
	proc.WorkerPool = worker.NewWorkerPool()

	return proc
}

// Begin will start the workers inside the WorkerPool
// responsible for the various tasks inside the program
// This includes: HTTP RESTful API (NYI), user interaction (NYI),
// import directory polling, title formatting (NYI), OMDB querying (NYI),
// and the FFMPEG formatting (NYI)
// This method will wait on the WaitGroup attached to the WorkerPool
func (p *Processor) Begin() error {
	importWakeupChan := make(chan int)
	titleWakeupChan := make(chan int)
	//omdbWakeupChan := make(chan int)
	//formatWakupChan := make(chan int)

	tickInterval := time.Duration(p.Config.Format.ImportDirTickDelay * int(time.Second))
	if tickInterval <= 0 {
		log.Panic("Failed to start PollingWorker - TickInterval is non-positive (make sure 'import_polling_delay' is set in your config)")
	}
	go func(source <-chan time.Time, target chan int) {
		for {
			<-source
			target <- 1
		}
	}(time.NewTicker(tickInterval).C, importWakeupChan)

	// Start some workers in the pool to handle
	// the import directory polling
	log.Printf("Config: %#v\n", p.Config.Concurrent)
	worker.NewPollingWorkers(p.WorkerPool, p.Config.Concurrent.Import, p.pollingWorkerTask, importWakeupChan)
	worker.NewTitleWorkers(p.WorkerPool, p.Config.Concurrent.Title, p.titleWorkerTask, titleWakeupChan)

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
func (p *Processor) PollInputSource() (bool, error) {
	log.Printf("Polling input source for new files")
	newItemsFound := false
	walkFunc := func(path string, dir fs.DirEntry, err error) error {
		if err != nil {
			log.Panicf("PollInputSource failed - %v\n", err.Error())
		}

		if !dir.IsDir() {
			v, err := dir.Info()
			if err != nil {
				log.Panicf("Failed to get FileInfo for path %v - %v\n", path, err.Error())
			}

			if new := p.Queue.HandleFile(path, v); new {
				newItemsFound = true
			}
		}

		return nil
	}

	filepath.WalkDir(p.Config.Format.ImportPath, walkFunc)
	return newItemsFound, nil
}

func (p *Processor) FormatTitle(title string) string {
	return title
}

func (p *Processor) NotifyWorkers(stage enum.PipelineStage) {

}
