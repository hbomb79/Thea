package processor

import (
	"fmt"
	"github.com/hbomb79/TPA/profile"
	"github.com/hbomb79/TPA/worker"
	_ "github.com/mitchellh/mapstructure"
	"sync"
	"time"
)

const DEFAULT_THREADS_REQUIRED int = 2

// Public Commander interface for use outside of this file/package
type Commander interface {
	Start() error
	SetWindowSize(int)
	SetThreadPoolSize(int)
	WakeupChan() chan int
	Instances() []CommanderTask
}

type CommanderTaskStatus int

const (
	PENDING CommanderTaskStatus = iota
	WORKING
	WAITING_FOR_RESOURCES
	FINISHED
	TROUBLED
)

type CommanderTask interface {
	Start(*Processor) error
	Item() *QueueItem
	ProfileTag() string
	TargetLabel() string
	Stop()
	ThreadsRequired() int
	Status() CommanderTaskStatus
	SetStatus(CommanderTaskStatus)
	Trouble() Trouble
	Important() bool
	Progress() interface{}
}

// taskData is a struct that encapsulates all data
// required to transcode an item with ffmpeg.
type taskData struct {
	targetLabel string
	profileTag  string
	item        *QueueItem
}

// ffmpegCommander is an implementation of the Commander interface
// which is used by TPA to handle the automatic allocation of resources
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

	// A link to the main TPA processor instances
	processor *Processor

	// Internal channel that threads can submit itemTarget's on
	// to have them spawned in to full ffmpegInstances
	spawnChannel chan *taskData

	// Mutex for use when code is reading/mutating instance information
	instanceLock sync.Mutex
}

// Start is the main entry point for the Commander. This method is blocking
// and will only return once the queueChangedChannel is closed (manually, or via
// the Stop method which is preferred). TODO: Stop method.
func (commander *ffmpegCommander) Start() error {
	for {
		select {
		case _ = <-commander.queueChangedChannel:
			// Outside queue has changed, perform injest
			go func() {
				commander.consumeNewTargets()
			}()
		case target := <-commander.spawnChannel:
			// A thread is requesting an itemTarget be spawned
			go func() {
				commander.instanceLock.Lock()

				fmt.Printf("[Commander] (+) Newly discovered target %#v\n", target)
				instance := newFfmpegInstance(target.item, target.profileTag, target.targetLabel)
				commander.instances = append(commander.instances, instance)

				commander.instanceLock.Unlock()
			}()
		case _ = <-time.NewTicker(time.Second * 1).C:
			// Run periodic checks over the targets to give feedback to the user.
			go func() {
				commander.runHealthChecks()
			}()
		}

		go func() {
			commander.tryStartInstances()
		}()
	}
}

// startInstance is an internal method that will attempt to start an ffmpegInstance,
// consuming the resources required and monitoring the instance for errors.
// Resources automatically freed upon completion.
// This method should only be called when the instanceLock mutex is acquired.
func (commander *ffmpegCommander) startInstance(instance CommanderTask) {
	commander.threadPoolUsed += instance.ThreadsRequired()
	instance.SetStatus(WORKING)
	go func() {
		err := instance.Start(commander.processor)
		if err != nil {
			commander.raiseTrouble(instance, err)
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
		if instance.Status() != PENDING && instance.Status() != WAITING_FOR_RESOURCES {
			// Instances that aren't either PENDING or WAITING_FOR_RESOURCES are
			// not of our concern
			continue
		} else if !canStart || (holdQueueForImportant && !instance.Important()) {
			// Insufficient resources, or we're holding all queue progress
			// until an important item is complete.
			instance.SetStatus(WAITING_FOR_RESOURCES)

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
			instance.SetStatus(WAITING_FOR_RESOURCES)
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
			commander.spawnChannel <- target
		}
	}
}

// extractItemsFromWindow scans over the processor queue, injesting items
// up to the limit of the sliding window defined (windowSize). Paused and completed
// items in the queue do not contribute to this window, and are skipped with no effect
// on the algorithm.
func (commander *ffmpegCommander) extractItemsFromWindow() []*QueueItem {
	items, itemsScanned := make([]*QueueItem, 0), 0

	commander.processor.Queue.ForEach(func(_ *processorQueue, index int, item *QueueItem) bool {
		if item.Status == Paused || item.Status == Completed {
			return false
		}

		itemsScanned++
		if item.Stage == worker.Format {
			items = append(items, item)
		}

		return itemsScanned == commander.windowSize
	})

	return items
}

// extractTargetsFromWindow is similar to extractItemsFromWindow, however it explodes each item
// down to their individual targets (as defined by each items profile). The result is a list
// of itemTarget instances that can be spawned via consumeNewTargets()
func (commander *ffmpegCommander) extractTargetsFromWindow() []*taskData {
	items, targets := commander.extractItemsFromWindow(), make([]*taskData, 0)

	for _, item := range items {
		profile := commander.selectBestProfile(item)
		if profile == nil {
			fmt.Printf("[Commander] (!!) QueueItem %s has invalid profile tag '%s'. Profile tag not found, cannot complete transcode!\n", item.Name, item.ProfileTag)
			continue
		}

		for _, target := range profile.Targets() {
			targets = append(targets, &taskData{target.Label, profile.Tag(), item})
		}
	}

	return targets
}

// selectBestProfile iterates over each TPA profile, checking to see which is
// the best fit for our QueueItem.
func (commander *ffmpegCommander) selectBestProfile(item *QueueItem) profile.Profile {
	if idx, p := commander.processor.Profiles.FindProfileByTag(item.ProfileTag); idx > 0 {
		return p
	}

	for _, profile := range commander.processor.Profiles.Profiles() {
		//TODO Filter profiles based on automatic-application filtering
		return profile
	}

	return nil
}

// runHealthChecks is an internal method that relays the current state of each item
// being processed by the commander, back to TPA by setting each items status. This method
// is important as it allows us to communicate to the user when a problem has arisen
func (commander *ffmpegCommander) runHealthChecks() {
	commander.instanceLock.Lock()
	defer commander.instanceLock.Unlock()

	// Create maps of both healthy and unhealthy instances we're aware of. Also create
	// a map of known items (by ID) so we know which items we're currently running instances for
	items, healthyInstances, unhealthyInstances := make(map[int]bool), make(map[int]int), make(map[int]int)
	for _, instance := range commander.instances {
		items[instance.Item().Id] = true

		if instance.Status() == TROUBLED {
			unhealthyInstances[instance.Item().Id]++
		} else if instance.Status() != FINISHED {
			healthyInstances[instance.Item().Id]++
		}
	}

	// Based on the above maps of known instances, interate over each, checking the counts of
	// healthy and unhealthy instances for each item. Using this information, we can adjust the status of
	// each QueueItem, or even identify those that are finished and advance their stage
	for id := range items {
		item, _ := commander.processor.Queue.FindById(id)
		if unhealthyInstances[id] == 0 {
			if healthyInstances[id] == 0 {
				commander.processor.Queue.AdvanceStage(item)
			} else {
				item.SetStatus(Processing)
			}
		} else {
			if healthyInstances[id] > 0 {
				item.SetStatus(NeedsAttention)
			} else {
				item.SetStatus(NeedsResolving)
			}
		}
	}
}

// raiseTrouble is an internal method that will raise a FormatTaskContainerError on
// the item that owns the instance provided (instance.context.item). If the item
// already has a FormatTaskContainerError assigned, this method will simply insert a new
// FormatTaskError into the container. Any other type of pre-existing trouble (including nil)
// will be replaced with a new container.
func (commander *ffmpegCommander) raiseTrouble(instance CommanderTask, err error) {
	fmt.Printf("[Trouble] (!!) Commander raising FormatTaskError for instance %v\n\t%s\n", instance, err.Error())
	item := instance.Item()

	var container *FormatTaskContainerError
	if item.Trouble != nil {
		v, ok := item.Trouble.(*FormatTaskContainerError)
		if !ok {
			// Incorrect trouble type - let's just overwrite it because that's easier
			fmt.Printf("[Commander] (!) Item's pre-existing Trouble is not of the correct type (%T, expected *FormatTaskContainerError). Overwriting!\n", item.Trouble)
			container = nil
		}

		container = v
	}

	if container == nil {
		container = &FormatTaskContainerError{NewBaseTaskError(err.Error(), item, FFMPEG_FAILURE), make([]Trouble, 0)}
	}

	container.Raise(&FormatTaskError{NewBaseTaskError(err.Error(), instance.Item(), FFMPEG_FAILURE), instance})
	item.SetTrouble(container)
	instance.SetStatus(TROUBLED)
}

// findTask accepts some taskData, and returns the CommanderTask that is working on it, and it's index. If
// no CommanderTask exists for this taskData, -1 and nil is returned (for index and instance respectively).
// Matching of taskData to CommanderTask is done per-field, the objects only need to contain the same
// data (QueueItem, profile and target) - they need not be identical objects (i.e. same address)
func (commander *ffmpegCommander) findTask(target *taskData) (int, CommanderTask) {
	for idx, instance := range commander.instances {
		if instance.Item().Id == target.item.Id && instance.ProfileTag() == target.profileTag && instance.TargetLabel() == target.targetLabel {
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

// NewCommander creates a new ffmpegCommander instance, with the channels
// already initialised for use.
func NewCommander(proc *Processor) Commander {
	return &ffmpegCommander{
		queueChangedChannel: make(chan int),
		spawnChannel:        make(chan *taskData),
		processor:           proc,
		instances:           make([]CommanderTask, 0),
	}
}
