package ffmpeg

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/asaskevich/EventBus"
	"github.com/floostack/transcoder"
	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/profile"
	"github.com/hbomb79/Thea/internal/queue"
	"github.com/hbomb79/Thea/pkg/logger"
)

var commanderLogger = logger.Get("Commander")

/**
 * FFmpeg Commander is a service that manages an items FFmpeg instances, from start to finish. This
 * includes detecting new items, spawning any missing FFmpeg instances, handling errors/trouble states,
 * managing system resouces, et cetera.
 *
 * Ultimately, the commander only needs to react to two things occuring:
 *  - New items being added to the Thea queue
 *  - Profiles changing
 *
 * When a new item is found in the queue, the commander will figure out which Thea profiles match this
 * item, and spawn FFmpeg instances for each - these instances will be spawned in an IDLE state, and will
 * wait for the go-ahead from the commander before they actually begin their processing (this is so we
 * can limit the number of ffmpeg instances occuring at any one time).
 *
 * As mentioned above, the commander uses the matching profiles to start the instances .. therefore if the profiles
 * change in any way, the commander will re-evaluate the instances. Note that at this stage a profile being removed
 * does NOT cancel any associatted instances, however new profiles will cause a new instance to be created (in an IDLE state, as above).
 */

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

const AVAILABLE_THREADS = 16
const DEFAULT_THREADS_REQUIRED = 1

type FfmpegCommander interface {
	Start(*sync.WaitGroup, context.Context) error
	CancelAllForItem(int)
	GetInstancesForItem(int) []FfmpegInstance
}

type Provider interface {
	GetItem(int) (*queue.QueueItem, error)
	GetAllItems() *[]*queue.QueueItem
	GetAllProfiles() []profile.Profile
	GetProfileByTag(string) profile.Profile
	EventBus() EventBus.BusSubscriber
	NotifyItemUpdate(int)
	NotifyFfmpegUpdate(int, FfmpegInstance)
	AdvanceItem(*queue.QueueItem)
}

type commander struct {
	provider          Provider
	itemInstances     map[int][]FfmpegInstance
	updateChan        chan uuid.UUID
	consumedThreads   uint32
	config            FormatterConfig
	lastKnownProgress map[uuid.UUID]transcoder.Progress
	queueChangeChan   chan bool
}

type FfmpegTask interface {
	Cancel()
	RequiredThreads() uint32
}

func (com *commander) Start(parentWg *sync.WaitGroup, ctx context.Context) error {
	defer func() {
		defer parentWg.Done()

		com.provider.EventBus().Unsubscribe("update:queue", com.queueEventHandler)
		com.provider.EventBus().Unsubscribe("update:item", com.queueEventHandler)
		com.provider.EventBus().Unsubscribe("update:profile", com.queueEventHandler)
		com.stop()
	}()

	com.provider.EventBus().SubscribeAsync("update:queue", com.queueEventHandler, true)
	com.provider.EventBus().SubscribeAsync("update:item", com.queueEventHandler, true)
	com.provider.EventBus().SubscribeAsync("update:profile", com.queueEventHandler, true)

	debouncedQueueChangeChannel := debounceChannel(time.Second*2, time.Second*5, com.queueChangeChan)

	commanderLogger.Emit(logger.NEW, "FFmpeg Commander Started\n")
	for {
		select {
		case instanceID := <-com.updateChan:
			com.HandleInstanceUpdate(instanceID)

		case <-debouncedQueueChangeChannel:
			com.IngestQueue()

		case <-ctx.Done():
			return nil
		}
	}
}

func (com *commander) IngestQueue() {
	commanderLogger.Emit(logger.INFO, "Handling queue/item update...\n")
	seenItems := make(map[int]bool)
	for _, item := range *com.provider.GetAllItems() {
		seenItems[item.ItemID] = true

		if item.Stage != queue.Format {
			continue
		}

		com.ingestItem(item)
	}

	commanderLogger.Emit(logger.INFO, "Queue/item update handled (saw %#v)\n", seenItems)

	// Detect items we have instances for that no
	// longer exist in the queue (abnormal state)
	for itemID, instances := range com.itemInstances {
		if _, ok := seenItems[itemID]; !ok {
			commanderLogger.Emit(logger.WARNING, "Found %v instances for item %v, however this item does not exist in the queue!\n", len(instances), itemID)
			com.CancelAllForItem(itemID)
		}
	}

	com.StartInstances()
}

// Notify the parent provider that this instance has changed - it's likely
// a progress update from FFmpeg, but could also be that the instance has
// been cancelled.
func (com *commander) HandleInstanceUpdate(instanceID uuid.UUID) {
	commanderLogger.Emit(logger.INFO, "Handling update for instance %v\n", instanceID)
	instance := com.getInstance(instanceID)
	instances := com.GetInstancesForItem(instance.ItemID())
	item, err := com.provider.GetItem(instance.ItemID())
	if err != nil {
		commanderLogger.Emit(logger.ERROR, "Failed to handle instance (%v) update because it's item (%v) could be fetched: %s\n", instanceID, instance.ItemID(), err.Error())
		return
	}

	count := len(instances)
	states := make(map[InstanceStatus]int)
	for _, v := range instances {
		states[v.Status()] = getOrDefault(states, v.Status(), 0) + 1
	}

	// Recalculate the items state. In order of priority, the following rules are used:
	// - ALL instances complete/cancelled -> *Advance to next stage* -> Pending
	// - ALL instances are troubled -> NeedsResolving
	// - One or more instances are troubled -> NeedsAttention
	// - ALL instances are paused -> Paused
	// - ALL instances are waiting -> Pending
	// - At least one instance is working -> Processing
	if getOrDefault(states, COMPLETE, 0)+getOrDefault(states, CANCELLED, 0) == count {
		com.provider.AdvanceItem(item)
	} else if getOrDefault(states, TROUBLED, 0) == count {
		item.SetStatus(queue.NeedsResolving)
	} else if getOrDefault(states, TROUBLED, 0) != 0 {
		item.SetStatus(queue.NeedsAttention)
	} else if getOrDefault(states, SUSPENDED, 0) == count {
		item.SetStatus(queue.Paused)
	} else if getOrDefault(states, WAITING, 0) == count {
		item.SetStatus(queue.Pending)
	} else if getOrDefault(states, WORKING, 0) != 0 {
		item.SetStatus(queue.Processing)
	} else {
		//TODO log unknown state
		commanderLogger.Emit(logger.WARNING, "Unexpected item state (item %v)... %#v\n", item.ItemID, states)
	}

	com.provider.NotifyItemUpdate(instance.ItemID())
	com.provider.NotifyFfmpegUpdate(item.ItemID, instance)
}

// getOrDefault accepts a map, a key, and a default value and will return
// the value for the key provided in the map (or the default if the key does
// not exist in the map)
func getOrDefault(m map[InstanceStatus]int, key InstanceStatus, def int) int {
	if prev, ok := m[key]; ok {
		return prev
	}

	return def
}

// StartInstances will inspect all instances in this commander and start any that are
// ready to be started. This is done in the same order as the item queue, and is typically
// performed after ingesting a new item, or after a running ffmpeg task finishes.
func (com *commander) StartInstances() {
	commanderLogger.Emit(logger.INFO, "Inspecting instances...\n")
	items := com.provider.GetAllItems()
	for _, item := range *items {
		instances, ok := com.itemInstances[item.ItemID]
		if !ok || item.Status == queue.Paused {
			// Not an item we know about, or it's paused - skip
			commanderLogger.Emit(logger.INFO, "Item %v is not eligble for instance start\n", item)
			continue
		}

		for _, instance := range instances {
			if instance.Status() != WAITING {
				// Instance is not waiting (so either busy, troubled or cancelled) - skip
				commanderLogger.Emit(logger.INFO, "Instance %v is NOT waiting (is %v)... skipping\n", instance.Id(), instance.Status())
				continue
			}

			requiredBudget, err := instance.RequiredThreads()
			if err != nil {
				commanderLogger.Emit(logger.ERROR, "Unable to get required threads for instance %v: %s\n", instance.Id(), err.Error())
				continue
			} else if requiredBudget > (AVAILABLE_THREADS - com.consumedThreads) {
				// No more budget, we've started all we can - finish
				return
			}

			// Waiting for resources... and we have sufficient budget - START!
			com.startInstance(instance, requiredBudget)
		}
	}
}

// startInstance will begin the execution and monitoring of this instance. Once an
// instance has started, it must only return once it's completely finished/cancelled. All
// trouble states should be handled by the instance directly. Once the instance returns, it's
// consumed resources will be released and re-allocated to future items
func (com *commander) startInstance(instance FfmpegInstance, threads uint32) {
	com.consumedThreads -= threads
	go func() {
		instance.Start(com.config, func(prog transcoder.Progress) {
			commanderLogger.Emit(logger.SUCCESS, "New progress for instance %v : %#v\n", instance.Id(), prog)
			com.updateChan <- instance.Id()
		})

		atomic.AddUint32(&com.consumedThreads, -threads)
	}()
}

func (com *commander) CancelAllForItem(itemID int) {
	instances, ok := com.itemInstances[itemID]
	if !ok {
		return
	}

	commanderLogger.Emit(logger.INFO, "Cancelling all tasks for item %s\n", itemID)
	for _, instance := range instances {
		instance.Cancel()
	}
}

func (com *commander) GetInstancesForItem(itemID int) []FfmpegInstance {
	if v, ok := com.itemInstances[itemID]; ok {
		return v
	}

	return nil
}

func (com *commander) stop() {
	commanderLogger.Emit(logger.STOP, "Cancelling all FFmpeg tasks...\n")
	for itemID := range com.itemInstances {
		com.CancelAllForItem(itemID)
	}
}

// ingestItem will create an instance for each Thea profile that the
// item is suitable for. This method does not cleanup any instances which have
// had their profile deleted. It also does not START the instance.
func (com *commander) ingestItem(item *queue.QueueItem) {
	// Get existing instances
	existingTaskProfileLabels := make(map[string]bool)
	if existingInstances, ok := com.itemInstances[item.ItemID]; ok {
		for _, instance := range existingInstances {
			existingTaskProfileLabels[instance.Profile()] = true
		}
	}

	// Find new instances, ignoring ones we already have running
	// Note that at this stage we are not stopping tasks who have had
	// their profiles deleted since their execution started.
	outputTasks := make([]FfmpegInstance, 0)
	for _, p := range com.provider.GetAllProfiles() {
		if _, ok := existingTaskProfileLabels[p.Tag()]; !ok && item.ValidateProfileSuitable(p) {
			outputTasks = append(outputTasks, NewFfmpegInstance(item.ItemID, p.Tag(), com.provider))
		}
	}

	if len(outputTasks) == 0 && len(existingTaskProfileLabels) == 0 {
		// Hmm.. nothing found for this item, raise a trouble as this is *likely* to be unexpected.
		item.SetTrouble(&queue.ProfileSelectionError{
			BaseTaskError: queue.NewBaseTaskError("No eligible Thea profiles were found. Please create an eligible profile, or update an existing one", item, queue.FFMPEG_FAILURE),
		})

		return
	} else if _, ok := item.Trouble.(*queue.ProfileSelectionError); ok {
		// Auto-resolve a profile selection trouble
		commanderLogger.Emit(logger.SUCCESS, "Automatically resolving ProfileSelectionError for item %s!\n", item)
		item.ClearTrouble()
	}

	// Append all new instances to the end of the list
	com.itemInstances[item.ItemID] = append(com.itemInstances[item.ItemID], outputTasks...)
}

func (com *commander) getInstance(instanceID uuid.UUID) FfmpegInstance {
	for _, instances := range com.itemInstances {
		for _, inst := range instances {
			if inst.Id() == instanceID {
				return inst
			}
		}
	}

	return nil
}

func (com *commander) queueEventHandler() {
	commanderLogger.Emit(logger.INFO, "Queue changed event!\n")
	com.queueChangeChan <- true
}

func NewFfmpegCommander(ctx context.Context, provider Provider, config FormatterConfig) FfmpegCommander {
	return &commander{
		provider:          provider,
		itemInstances:     make(map[int][]FfmpegInstance),
		lastKnownProgress: make(map[uuid.UUID]transcoder.Progress),
		updateChan:        make(chan uuid.UUID),
		queueChangeChan:   make(chan bool),
		config:            config,
	}
}

// debounceChannel performs debounce filtering of events emitting on the input chan, and outputs
// acceptable messages on the returned channel. If the input channel is given a un-steady stream of events,
// the output channel will output one message once a break in the messages of atleast 'min' time occurs.
// If the input channel is receiving a steady-stream of messages (with an interval < min), then the max time window can
// be used to force a message to be emitted on the output channel atleast once per 'max' time duration.
// Source: https://gist.github.com/gigablah/80d7160f3577edc153c9
func debounceChannel[T any](min time.Duration, max time.Duration, input chan T) chan T {
	output := make(chan T)

	go func() {
		var (
			buffer   T
			ok       bool
			minTimer <-chan time.Time
			maxTimer <-chan time.Time
		)

		// Start debouncing
		for {
			select {
			case buffer, ok = <-input:
				if !ok {
					return
				}

				minTimer = time.After(min)
				if maxTimer == nil {
					maxTimer = time.After(max)
				}
			case <-minTimer:
				minTimer, maxTimer = nil, nil
				output <- buffer
			case <-maxTimer:
				minTimer, maxTimer = nil, nil
				output <- buffer
			}
		}
	}()

	return output
}
