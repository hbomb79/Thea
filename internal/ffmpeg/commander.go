package ffmpeg

import (
	"fmt"
	"sync"
	"time"

	"github.com/hbomb79/Thea/internal/profile"
	"github.com/hbomb79/Thea/internal/queue"
	"github.com/hbomb79/Thea/pkg/logger"
)

var commanderLogger = logger.Get("Commander")

// FfmpegManager interface for use outside of this file/package
type FfmpegManager interface {
	Start(*sync.WaitGroup) error
	Stop()
	SetWindowSize(int)
	SetThreadPoolSize(int)
	WakeupChan() chan int
	Instances() []CommanderTask
	GetInstancesForItem(int) []CommanderTask
}

type Supplier interface {
	GetAllItems() *[]*queue.QueueItem
	GetAllProfiles() []profile.Profile
	AdvanceItem(*queue.QueueItem)
}

// FormatterConfig is the 'misc' container of the configuration, encompassing configuration
// not covered by either 'ConcurrentConfig' or 'DatabaseConfig'. Mainly configuration
// paramters for the FFmpeg executable.
type FormatterConfig struct {
	ImportPath         string `yaml:"import_path" env:"FORMAT_IMPORT_PATH" env-required:"true"`
	OutputPath         string `yaml:"default_output_dir" env:"FORMAT_DEFAULT_OUTPUT_DIR" env-required:"true"`
	TargetFormat       string `yaml:"target_format" env:"FORMAT_TARGET_FORMAT" env-default:"mp4"`
	ImportDirTickDelay int    `yaml:"import_polling_delay" env:"FORMAT_IMPORT_POLLING_DELAY" env-default:"3600"`
	FfmpegBinaryPath   string `yaml:"ffmpeg_binary" env:"FORMAT_FFMPEG_BINARY_PATH" env-default:"/usr/bin/ffmpeg"`
	FfprobeBinaryPath  string `yaml:"ffprobe_binary" env:"FORMAT_FFPROBE_BINARY_PATH" env-default:"/usr/bin/ffprobe"`
}

// CommanderTaskStatus is used as the data type/enum for the status of
// tasks/processes that the Commander is/was managing
type CommanderTaskStatus int

const (
	// PENDING means a task has been created but not started or allocated to any worker
	PENDING CommanderTaskStatus = iota

	// WORKING indicates a task is now in progress
	WORKING

	// WAITING means a task has been created however, insufficient resources are available, therefore
	// the Commander will wait for sufficient resources to be available before starting this task
	WAITING

	FINISHED

	CANCELLED

	// TROUBLED tasks require intervention. Inspect 'Trouble' of task via Trouble() method
	TROUBLED

	PAUSED
)

// CommanderTask is the basic interface of any tasks that the Commander operates
// on.
type CommanderTask interface {
	Start(FormatterConfig)
	Item() *queue.QueueItem
	ProfileTag() string
	Stop()
	ThreadsRequired() int
	Status() CommanderTaskStatus
	SetStatus(CommanderTaskStatus)
	SetPaused(bool)
	Trouble() queue.Trouble
	ResolveTrouble(map[string]interface{}) error
	Important() bool
	Progress() interface{}
	GetOutputPath() string
}

// taskData is a struct that encapsulates all data
// required to transcode an item with ffmpeg.
type taskData struct {
	profile profile.Profile
	item    *queue.QueueItem
}

// ffmpegCommander is an implementation of the Commander interface
// which is used by Thea to handle the automatic allocation of resources
// for FFmpeg instances, as well as Trouble/error handling.
type ffmpegCommander struct {
	// Current ffmpeg instances. Check their 'state' to see if running, waiting, troubled, etc
	instances []CommanderTask

	// The size of the 'sliding window' the Commander searches through
	// when injesting new items (and targets)
	windowSize int

	// The amount of threads we're willing to allocate to ffmpeg instances
	threadPoolSize int

	// The amount of threads allocated to ffmpeg instances. Must not exceed threadPoolSize
	threadPoolUsed int

	// A channel that is made publicly available via 'WakeupChan()' that
	// can be used to tell the Commander that the queue has changed and we should
	// re-evaluate state
	queueChangedChannel chan int

	// A link to the main Thea instances
	supplier Supplier

	// Mutex for use when code is reading/mutating instance information
	instanceLock sync.Mutex

	healthTicker time.Ticker

	doneChannel chan int

	config FormatterConfig
}

// Start is the main entry point for the Commander. This method is blocking
// and will only return once the commander has finished (by calling Stop())
func (commander *ffmpegCommander) Start(parentWg *sync.WaitGroup) error {
	defer parentWg.Done()
	commanderLogger.Emit(logger.NEW, "Listening on all data channels.\n")
	wg := &sync.WaitGroup{}
main:
	for {
		select {
		case <-commander.queueChangedChannel:
			// Outside queue has changed, perform ingest
			wg.Add(1)
			go func() {
				defer wg.Done()
				commander.consumeNewTargets()
			}()
		case <-commander.healthTicker.C:
			// Run periodic checks over the targets to give feedback to the user.
			wg.Add(1)
			go func() {
				defer wg.Done()
				commander.runHealthChecks()
			}()
		case <-commander.doneChannel:
			commanderLogger.Emit(logger.STOP, "STOP signal received!\n")
			break main
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			commander.tryStartInstances()
		}()
	}

	commanderLogger.Emit(logger.STOP, "Channel loop terminated - Waiting for running actions to stop...\n")
	wg.Wait()
	commanderLogger.Emit(logger.STOP, "Terminating ffmpeg instances...\n")
	for _, ffmpegI := range commander.instances {
		ffmpegI.Stop()
	}

	return nil
}

// Stop will stop all activities from the commander by terminating each ffmpeg instance
func (commander *ffmpegCommander) Stop() {
	close(commander.doneChannel)
}

// startInstance is an internal method that will attempt to start an ffmpegInstance,
// consuming the resources required and monitoring the instance for errors.
// Resources automatically freed upon completion.
// This method should only be called when the instanceLock mutex is acquired.
func (commander *ffmpegCommander) startInstance(instance CommanderTask) {
	commander.threadPoolUsed += instance.ThreadsRequired()
	instance.SetStatus(WORKING)
	go func() {
		instance.Start(commander.config)

		if instance.Status() == CANCELLED {
			//TODO Perform cleanup of partially formatted content
		} else {
			commanderLogger.Emit(logger.WARNING, "FFMPEG instance %v exited with abormal state %v!\n", instance, instance.Status())
		}

		// Whatever resources this instance had are now freed. Critical section, protect with mutex
		commander.instanceLock.Lock()
		commander.threadPoolUsed -= instance.ThreadsRequired()
		commander.instanceLock.Unlock()
	}()
}

// tryStartInstances is an internal method that scans through the list of instances
// and attempts to start any that are currently pending or waiting for resources.
// Instances of any other type are skipped, and pending instances are only started (via startInstance)
// if the required amount of resources are available. Once resources are depleted,
// all instances are marked as 'WAITING_FOR_RESOURCES'
func (commander *ffmpegCommander) tryStartInstances() {
	commander.instanceLock.Lock()
	defer commander.instanceLock.Unlock()

	// Scan for an 'important' instance in the queue
	holdQueueForImportant := false
	for _, instance := range commander.instances {
		if instance.Important() {
			holdQueueForImportant = true
			break
		}
	}

	canStart := true
	for _, instance := range commander.instances {
		if instance.Status() != PENDING && instance.Status() != WAITING {
			// Instances that aren't either PENDING or WAITING_FOR_RESOURCES are
			// not of our concern
			continue
		} else if !canStart || (holdQueueForImportant && !instance.Important()) {
			// Insufficient resources, or we're holding all queue progress
			// until an important item is complete.
			instance.SetStatus(WAITING)

			continue
		}

		if holdQueueForImportant {
			if commander.threadPoolUsed != 0 {
				// An important item requires 100% of the thread pool. We don't have that currently
				// so continue holding other items until we complete this important item
				canStart = false
			}
		} else if commander.threadPoolSize-(commander.threadPoolUsed+instance.ThreadsRequired()) < 0 {
			// Normal queue function, check if we have available resources for this instance
			canStart = false
		}
		if !canStart {
			// Above logic concluded not enough resources. Mark instances as such and continue to next item
			instance.SetStatus(WAITING)
			continue
		}

		commander.startInstance(instance)
	}
}

// consumeNewTargets spawns new ffmpegInstances for targets from the commanders sliding
// window, providing the targets are not already known to the commander.
func (commander *ffmpegCommander) consumeNewTargets() {
	commander.instanceLock.Lock()
	defer commander.instanceLock.Unlock()

	targets := commander.extractTargetsFromWindow()

	// Spawn targets we don't recognize
	for _, target := range targets {
		if instanceIdx, _ := commander.findTask(target); instanceIdx == -1 {
			commanderLogger.Emit(logger.NEW, "Newly discovered target {%s %s}\n", target.profile.Tag(), target.item)
			instance := newFfmpegInstance(target.item, target.profile)
			commander.instances = append(commander.instances, instance)
		}
	}
}

// extractItemsFromWindow scans over the processor queue, injesting items
// up to the limit of the sliding window defined (windowSize). Paused, troubled and completed
// items in the queue do not contribute to this window, and are skipped with no effect
// on the algorithm.
func (commander *ffmpegCommander) extractItemsFromWindow() []*queue.QueueItem {
	outputItems, itemsScanned := make([]*queue.QueueItem, 0), 0

	items := commander.supplier.GetAllItems()
	for _, item := range *items {
		if item.Status == queue.Paused || item.Status == queue.Completed {
			continue
		}

		itemsScanned++
		if item.Stage == queue.Format {
			if t := item.Trouble; t != nil {
				// Item is troubled, if the user has provided a resolution then we can address it - otherwise
				// we treat this item as if it doesn't exist (does not contribute to itemsScanned)
				if len(t.ResolutionContext()) == 0 {
					continue
				}

				item.ClearTrouble()
				item.SetStatus(queue.Pending)
			}

			outputItems = append(outputItems, item)
		}

		if itemsScanned >= commander.windowSize {
			break
		}
	}

	return outputItems
}

// extractTargetsFromWindow is similar to extractItemsFromWindow, however it explodes each item
// down to their individual targets (as defined by each items profile). The result is a list
// of itemTarget instances that can be spawned via consumeNewTargets()
func (commander *ffmpegCommander) extractTargetsFromWindow() []*taskData {
	items, targets := commander.extractItemsFromWindow(), make([]*taskData, 0)

	for _, item := range items {
		profiles, err := commander.selectMatchingProfiles(item)
		if err != nil {
			if item.Trouble == nil {
				commanderLogger.Emit(logger.ERROR, "Profile selection failed for item %s: %s\n", item.Name, err.Error())
				item.SetTrouble(&queue.ProfileSelectionError{BaseTaskError: queue.NewBaseTaskError(fmt.Sprintf("Profile selection failed - %s", err.Error()), item, queue.COMMANDER_FAILURE)})
			}

			continue
		}

		for _, p := range profiles {
			targets = append(targets, &taskData{p, item})
		}
	}

	return targets
}

// selectMatchingProfiles iterates over each Thea profile, checking to see which is
// the best fit for our QueueItem.
func (commander *ffmpegCommander) selectMatchingProfiles(item *queue.QueueItem) ([]profile.Profile, error) {
	output := make([]profile.Profile, 0)
	profiles := commander.supplier.GetAllProfiles()
	if len(profiles) == 0 {
		return nil, fmt.Errorf("cannot perform profile selection for item %s because server has NO profiles", item)
	}

	for _, profile := range profiles {
		if item.ValidateProfileSuitable(profile) {
			output = append(output, profile)
		}
	}

	return output, nil
}

// runHealthChecks is an internal method that relays the current state of each item
// being processed by the commander, back to Thea by setting each items status. This method
// is important as it allows us to communicate to the user when a problem has arisen
func (commander *ffmpegCommander) runHealthChecks() {
	commander.instanceLock.Lock()
	defer commander.instanceLock.Unlock()

	// Create maps of both healthy and unhealthy instances we're aware of. Also create
	// a map of known items (by ID) so we know which items we're currently running instances for
	items, healthyInstances, unhealthyInstances := make(map[int]bool), make(map[int]int), make(map[int]int)
	for _, instance := range commander.instances {
		items[instance.Item().ItemID] = true

		if instance.Status() == TROUBLED {
			unhealthyInstances[instance.Item().ItemID]++
		} else if instance.Status() != FINISHED {
			healthyInstances[instance.Item().ItemID]++
		}
	}

	// Based on the above maps of known instances, interate over each item in this stage, checking the counts of
	// healthy and unhealthy instances for each item. Using this information, we can adjust the status of
	// each QueueItem, or even identify those that are finished and advance their stage
	for _, item := range commander.extractItemsFromWindow() {
		id := item.ItemID
		if unhealthyInstances[id] == 0 {
			if healthyInstances[id] == 0 {
				// Before advancing we should check to make sure that the reason this item appears finished
				// is because it actually hasn't _started_ yet. This is certain to be the case when an item
				// is troubled.
				if item.Trouble != nil {
					continue
				}

				commander.supplier.AdvanceItem(item)
			} else {
				item.SetStatus(queue.Processing)
			}
		} else {
			if healthyInstances[id] > 0 {
				item.SetStatus(queue.NeedsAttention)
			} else {
				item.SetStatus(queue.NeedsResolving)
			}
		}
	}
}

// findTask accepts some taskData, and returns the CommanderTask that is working on it, and it's index. If
// no CommanderTask exists for this taskData, -1 and nil is returned (for index and instance respectively).
// Matching of taskData to CommanderTask is done per-field, the objects only need to contain the same
// data (QueueItem, profile and target) - they need not be identical objects (i.e. same address)
func (commander *ffmpegCommander) findTask(target *taskData) (int, CommanderTask) {
	for idx, instance := range commander.instances {
		if instance.Item().ItemID == target.item.ItemID && instance.ProfileTag() == target.profile.Tag() {
			return idx, instance
		}
	}
	return -1, nil
}

// SetWindowSize allows the user to set how large our sliding window is for items in the queue.
// Essentially this property controls how many items we can injest (and therefore process) at once.
func (commander *ffmpegCommander) SetWindowSize(size int) {
	commander.windowSize = size
}

// SetThreadPoolSize sets the maximum amount of resources (threads) available to use.
func (commander *ffmpegCommander) SetThreadPoolSize(threads int) {
	commander.threadPoolSize = threads
}

// WakeupChan is the public accessor for the Commanders wakeup channel. Sending
// an int on this channel will notify that the queue contents have changed, prompting
// the commander to rescan it's sliding window for new targets to injest
func (commander *ffmpegCommander) WakeupChan() chan int {
	return commander.queueChangedChannel
}

// Instances returns the array of ffmpegInstances currently under this commanders control
func (commander *ffmpegCommander) Instances() []CommanderTask {
	return commander.instances
}

// GetInstancesForItem will return all matching CommanderTasks for a specified ID
func (commander *ffmpegCommander) GetInstancesForItem(ID int) []CommanderTask {
	out := make([]CommanderTask, 0)

	for _, instance := range commander.instances {
		if instance.Item().ItemID == ID {
			out = append(out, instance)
		}
	}

	return out
}

// NewCommander creates a new ffmpegCommander instance, with the channels
// already initialised for use.
func NewCommander(supplier Supplier, config FormatterConfig) FfmpegManager {
	return &ffmpegCommander{
		queueChangedChannel: make(chan int),
		supplier:            supplier,
		instances:           make([]CommanderTask, 0),
		healthTicker:        *time.NewTicker(time.Second * 2),
		doneChannel:         make(chan int),
		config:              config,
	}
}
