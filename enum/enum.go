package enum

// Each stage represents a certain stage in the pipeline
type PipelineStage int
type WorkerWakeupChan chan int

// When a QueueItem is initially added, it should be of stage Import,
// each time a worker works on the task it should increment it's
// Stage (Title->OMDB->etc..) and set it's Status to 'Pending'
// to allow a worker to pick the item from the Queue
const (
	Import PipelineStage = iota
	Title
	OMDB
	Format
	Finish
)

// QueueItemStatus represents whether or not the
// QueueItem is currently being worked on, or if
// it's waiting for a worker to pick it up
// and begin working on the task
type QueueItemStatus int

// If a task is Pending, it's waiting for a worker
// ... if processing, it's currently being worked on.
// When a stage in the pipeline is finished with the task,
// it should set the Stage to the next stage, and set the
// Status to pending - except for Format stage, which should
// mark it as completed
const (
	Pending QueueItemStatus = iota
	Processing
	Completed
)

type WorkerStatus int

const (
	Idle WorkerStatus = iota
	Working
)

type QueueItem struct {
	Path   string
	Name   string
	Status QueueItemStatus
	Stage  PipelineStage
}
