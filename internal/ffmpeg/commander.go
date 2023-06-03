package ffmpeg

import (
	"sync/atomic"
	"time"

	"github.com/floostack/transcoder"
	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/event"
	"github.com/hbomb79/Thea/internal/profile"
	"github.com/hbomb79/Thea/internal/queue"
	"github.com/hbomb79/Thea/pkg/logger"
)

var log = logger.Get("Commander")

/*
 * FFmpeg Commander is a service that manages an items FFmpeg instances, from start to finish. This
 * includes detecting new items, spawning any missing FFmpeg instances, handling errors/trouble states,
 * managing system resouces, et cetera.
 *
 * Ultimately, the commander only needs to react to:
 *  - Items changing in the Thea queue (we're only interested in items at the Format stage)
 *  - Profiles changing
 *	- FFmpeg instance updates (transcoding starting/finishing)
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

const (
	EXTERNAL_UPDATE_CHANNEL_BUFFER = 10
	AVAILABLE_THREADS              = 16
	DEFAULT_THREADS_REQUIRED       = 1
)

type FfmpegCommander interface {
	Start()
	Stop()
	CancelAllForItem(int)
	GetInstancesForItem(int) []FfmpegInstance
}

type Provider interface {
	GetItem(int) (*queue.Item, error)
	GetAllItems() *[]*queue.Item
	GetAllProfiles() []profile.Profile
	GetProfileByTag(string) profile.Profile
	EventHandler() event.EventHandler
	NotifyItemUpdate(int)
	NotifyFfmpegUpdate(int)
	AdvanceItem(*queue.Item)
}

type commander struct {
	provider           Provider
	itemInstances      map[int][]FfmpegInstance
	updateChan         chan uuid.UUID
	consumedThreads    uint32
	config             FormatterConfig
	lastKnownProgress  map[uuid.UUID]transcoder.Progress
	externalChangeChan event.HandlerChannel
	exitChan           chan bool
}

type FfmpegTask interface {
	Cancel()
	RequiredThreads() uint32
}

func NewFfmpegCommander(provider Provider, config FormatterConfig) FfmpegCommander {
	return &commander{
		provider:           provider,
		itemInstances:      make(map[int][]FfmpegInstance),
		lastKnownProgress:  make(map[uuid.UUID]transcoder.Progress),
		updateChan:         make(chan uuid.UUID),
		externalChangeChan: make(event.HandlerChannel, EXTERNAL_UPDATE_CHANNEL_BUFFER),
		exitChan:           make(chan bool),
		config:             config,
	}
}

func (com *commander) Start() {
	defer com.stop()

	// Subscribe to event bus and forward incoming events to the queue changed channel
	eventBus := com.provider.EventHandler()
	eventBus.RegisterHandlerChannel(event.QUEUE_UPDATE_EVENT, com.externalChangeChan)
	eventBus.RegisterHandlerChannel(event.ITEM_UPDATE_EVENT, com.externalChangeChan)
	eventBus.RegisterHandlerChannel(event.PROFILE_UPDATE_EVENT, com.externalChangeChan)

	// Debounce the queue change channel so that we don't do repeat-work for no reason.
	debouncedQueueChangeChannel := debounceEventChannel(time.Second*2, time.Second*5, com.externalChangeChan)

	log.Emit(logger.NEW, "FFmpeg Commander Started\n")
	for {
		select {
		case instanceID := <-com.updateChan:
			// Internal changes detected from FFmpeg instance
			// E.g: Completed, cancelled, troubled, or a transcode progress update
			com.handleFFmpegInstanceChange(instanceID)

		case <-debouncedQueueChangeChannel:
			// External changes detected from event bus which may require us to ingest new
			// items, or prune existing instances for items that have gone away
			// E.g. New items in Thea queue, or items/profiles have changed
			com.synchronizeQueue()

		case <-com.exitChan:
			return
		}
	}
}

// Stop will close this commander by closing an internal channel and causing
// the channel read loop in `Start` to finish.
func (com *commander) Stop() {
	close(com.exitChan)
}

func (com *commander) GetInstancesForItem(itemID int) []FfmpegInstance {
	if v, ok := com.itemInstances[itemID]; ok {
		return v
	}

	return nil
}

func (com *commander) CancelAllForItem(itemID int) {
	instances, ok := com.itemInstances[itemID]
	if !ok {
		return
	}

	log.Emit(logger.INFO, "Cancelling all tasks for item %v\n", itemID)
	for _, instance := range instances {
		instance.Cancel()
	}
}

// synchronizeQueue crawls through the provider Queue in response to an item/queue change
// being detected. All items are ingested, and any previously ingested items that
// no longer appear in the provider list will have their instances cancelled.
func (com *commander) synchronizeQueue() {
	log.Emit(logger.DEBUG, "Handling queue/item update...\n")
	seenItems := make(map[int]bool)
	for _, item := range *com.provider.GetAllItems() {
		seenItems[item.ItemID] = true

		if item.Stage != queue.Format {
			continue
		}

		com.ingestItem(item)
	}

	// Detect items we have instances for that no
	// longer exist in the queue (abnormal state)
	for itemID, instances := range com.itemInstances {
		if _, ok := seenItems[itemID]; !ok {
			log.Emit(logger.WARNING, "Found %v instance(s) for item %v, however this item does not exist in the queue!\n", len(instances), itemID)
			com.CancelAllForItem(itemID)
		}
	}

	log.Emit(logger.VERBOSE, "Queue/item update handled (saw %#v)\n", seenItems)
	com.startInstances()
}

// handleFFmpegInstanceChange notifies the parent provider that this instance has changed
// and sets the associatted item to the correct Status. This method is also responsible
// for detecting when an item has completed it's processing and moving this item on to
// the next stage.
func (com *commander) handleFFmpegInstanceChange(instanceID uuid.UUID) {
	log.Emit(logger.DEBUG, "Handling update for instance %v\n", instanceID)
	instance := com.getInstance(instanceID)
	instances := com.GetInstancesForItem(instance.ItemID())
	item, err := com.provider.GetItem(instance.ItemID())
	if err != nil {
		log.Emit(logger.ERROR, "Failed to handle instance (%v) update because it's item (%v) could be fetched: %s\n", instanceID, instance.ItemID(), err.Error())
		return
	}

	count := len(instances)
	states := make(map[InstanceStatus]int)
	for _, v := range instances {
		states[v.Status()] = getOrDefault(states, v.Status(), 0) + 1
	}
	log.Emit(logger.DEBUG, "Instance status' for item %v: %v\n", item, states)

	// Recalculate the items Status by checking the following scenarios (in order):
	// - Item is cancelling and ALL instances are cancelled -> Cancelled
	// - ALL instances "finished" -> *Advance to next stage* -> Pending
	// - ALL instances are troubled -> NeedsResolving
	// - One or more instances are troubled -> NeedsAttention
	// - ALL instances are paused -> Paused
	// - ALL instances are waiting -> Pending
	// - At least one instance is working -> Processing
	if item.Status == queue.Cancelling && getOrDefault(states, CANCELLED, 0) == count {
		item.SetStatus(queue.Cancelled)
	} else if getOrDefault(states, COMPLETE, 0)+getOrDefault(states, CANCELLED, 0) == count {
		com.provider.AdvanceItem(item)
	} else if troubled := getOrDefault(states, TROUBLED, 0); troubled != 0 {
		if troubled == count {
			item.SetStatus(queue.NeedsResolving)
		} else {
			item.SetStatus(queue.NeedsAttention)
		}
	} else if getOrDefault(states, SUSPENDED, 0) == count {
		item.SetStatus(queue.Paused)
	} else if getOrDefault(states, WAITING, 0) == count {
		item.SetStatus(queue.Pending)
	} else if getOrDefault(states, WORKING, 0) != 0 {
		item.SetStatus(queue.Processing)
	} else {
		log.Emit(logger.WARNING, "Unexpected item state (item %v)... %#v\n", item.ItemID, states)
	}

	com.provider.NotifyFfmpegUpdate(item.ItemID)
}

// startInstances will inspect all instances in this commander and start any that are
// ready to be started. This is done in the same order as the item queue, and is typically
// performed after ingesting a new item, or after a running ffmpeg task finishes.
func (com *commander) startInstances() {
	log.Emit(logger.DEBUG, "Inspecting instances...\n")
	items := com.provider.GetAllItems()
	for _, item := range *items {
		instances, ok := com.itemInstances[item.ItemID]
		if !ok || item.Status == queue.Paused {
			// Not an item we know about, or it's paused - skip
			log.Emit(logger.DEBUG, "Item %v is not eligble for instance start\n", item)
			continue
		}

		for _, instance := range instances {
			if instance.Status() != WAITING {
				// Instance is not waiting (so either busy, troubled or cancelled) - skip
				log.Emit(logger.DEBUG, "Instance %v is NOT waiting (is %v)... skipping\n", instance, instance.Status())
				continue
			}

			requiredBudget, err := instance.RequiredThreads()
			availableBudget := AVAILABLE_THREADS - com.consumedThreads
			if err != nil {
				log.Emit(logger.ERROR, "Unable to get required threads for instance %v: %s\n", instance, err.Error())
				continue
			} else if requiredBudget > availableBudget {
				// No more budget, we've started all we can - finish
				log.Emit(logger.DEBUG, "Thread requirements of instance %v (%v) exceed remaining budget (%v), instance spawning complete\n", instance, requiredBudget, availableBudget)
				return
			}

			// Waiting for resources... and we have sufficient budget - START!
			log.Emit(logger.DEBUG, "Sufficient budget to start instance %v, starting instance!\n", instance, item)
			com.startInstance(instance, requiredBudget)
		}
	}
}

// startInstance will begin the execution and monitoring of this instance. Once an
// instance has started, it must only return once it's completely finished/cancelled. All
// trouble states should be handled by the instance directly. Once the instance returns, it's
// consumed resources will be released and re-allocated to future items
func (com *commander) startInstance(instance FfmpegInstance, threads uint32) {
	atomic.AddUint32(&com.consumedThreads, threads)

	progressChan := make(ProgressChannel)
	go func() {
		for {
			prog, ok := <-progressChan
			com.updateChan <- instance.Id()
			if !ok {
				log.Emit(logger.DEBUG, "FFmpeg instance %v has closed progress channel!\n", instance)
				return
			}

			log.Emit(logger.VERBOSE, "New progress for instance %v : %#v\n", instance, prog)
		}
	}()

	go func() {
		defer atomic.AddUint32(&com.consumedThreads, -threads)
		instance.Start(com.config, progressChan)
	}()
}

func (com *commander) stop() {
	log.Emit(logger.STOP, "COMMANDER SHUTDOWN - Cancelling all FFmpeg instances...\n")
	for itemID := range com.itemInstances {
		com.CancelAllForItem(itemID)
	}
}

// ingestItem will create an instance for each Thea profile that the
// item is suitable for. This method does not cleanup any instances which have
// had their profile deleted. It also does not START the instance.
func (com *commander) ingestItem(item *queue.Item) {
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
		log.Emit(logger.SUCCESS, "Automatically resolving ProfileSelectionError for item %s!\n", item)
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

// debounceChannel performs debounce filtering of events emitting on the input chan, and outputs
// acceptable messages on the returned channel. If the input channel is given a un-steady stream of events,
// the output channel will output one message once a break in the messages of atleast 'min' time occurs.
// If the input channel is receiving a steady-stream of messages (with an interval < min), then the max time window can
// be used to force a message to be emitted on the output channel atleast once per 'max' time duration.
// Source: https://gist.github.com/gigablah/80d7160f3577edc153c9
func debounceEventChannel(min time.Duration, max time.Duration, source event.HandlerChannel) <-chan bool {
	output := make(chan bool)

	go func() {
		var (
			minTimer <-chan time.Time
			maxTimer <-chan time.Time
		)

		// Start debouncing
		for {
			select {
			case event, ok := <-source:
				if !ok {
					return
				}

				log.Emit(logger.DEBUG, "Received external event %v (%v)\n", event.Event, event.Payload)
				minTimer = time.After(min)
				if maxTimer == nil {
					maxTimer = time.After(max)
				}
			case <-minTimer:
				minTimer, maxTimer = nil, nil
				output <- true
			case <-maxTimer:
				minTimer, maxTimer = nil, nil
				output <- true
			}
		}
	}()

	return output
}

// getOrDefault accepts a map, a key, and a default value and will return
// the value for the key provided in the map (or the default if the key does
// not exist in the map)
func getOrDefault[T any](m map[InstanceStatus]T, key InstanceStatus, def T) T {
	if prev, ok := m[key]; ok {
		return prev
	}

	return def
}
