import { Writable, writable } from "svelte/store"
import { commander, dataStream } from "./commander"
import { SocketData, SocketMessageType, SocketPacketType, SocketStreamPacket, socketStream } from "./stores/socket"

import { ffmpegProfiles } from "stores/profiles";

import { itemIndex, itemDetails, itemFfmpegInstances } from "./stores/queue"
import { QueueOrderManager as QueueOrderManager } from "./queueOrderManager"

export enum QueueStatus {
    PENDING,
    PROCESSING,
    COMPLETED,
    NEEDS_RESOLVING,
    CANCELLING,
    CANCELLED,
    PAUSED,
    NEEDS_ATTENTION
}

export enum QueueStage {
    IMPORT,
    TITLE,
    OMDB,
    FFMPEG,
    DB,
    FINISH
}

export enum CommanderTaskStatus {
    WAITING,
    WORKING,
    SUSPENDED,
    TROUBLED,
    CANCELLED,
    COMPLETE
}

export enum QueueTroubleType {
    TITLE_FAILURE,
    OMDB_NO_RESULT_FAILURE,
    OMDB_MULTIPLE_RESULT_FAILURE,
    OMDB_REQUEST_FAILURE,
    FFMPEG_FAILURE,
}

export enum MatchKey {
    TITLE,
    RESOLUTION,
    SEASON_NUMBER,
    EPISODE_NUMBER,
    SOURCE_PATH,
    SOURCE_NAME,
    SOURCE_EXTENSION
}

export enum MatchType {
    EQUALS,
    NOT_EQUALS,
    MATCHES,
    DOES_NOT_MATCH,
    LESS_THAN,
    GREATER_THAN,
    IS_PRESENT,
    IS_NOT_PRESENT
}

export enum ModifierType {
    AND,
    OR
}

export interface ProfileMatchCriterion {
    key: MatchKey
    matchType: MatchType
    modifier: ModifierType
    matchTarget: any
}

export interface TranscodeProfile {
    tag: string
    outputPath: string
    matchCriteria: ProfileMatchCriterion[]
    command: Map<string, any>
    blocking: boolean
}

export interface TranscodeTarget {
    label: string
}

export interface CommanderTask {
    id: string
    progress: FfmpegProgress
    status: CommanderTaskStatus
    trouble: QueueTroubleDetails
}

export interface FfmpegProgress {
    Frames: string
    Elapsed: string
    Bitrate: string
    Progress: number
    Speed: string
}

export interface QueueItem {
    id: number
    name: string
    stage: QueueStage
    status: QueueStatus
    statusLine: string
    title_info: QueueTitleInfo
    omdb_info: QueueOmdbInfo
    trouble: QueueTroubleDetails
}

// QueueTroubleInfo represents the data we receive
// from the Go server regarding the Title info for
// our queue item. This information will be
// unavailable until the title formatting
// has been completed.
export interface QueueTitleInfo {
    Title: string
    Episodic: boolean
    Episode: number
    Season: number
    Year: number
    Resolution: string
}

// QueueOmdbInfo represents the information about a queue item
// from the go server - this data will be unavailable until
// a worker on the go server has queried OMDB for this information
export interface QueueOmdbInfo {
    Title: string
    plot: string
    ReleaseYear: number
    Runtime: string
    Type: string
    poster: string
    ImdbId: string
    Response: boolean
    Error: string
    Genre: string[]
}

export interface QueueTroubleDetails {
    message: string
    expected_args: any
    type: QueueTroubleType
    payload: any
    item_id: number
}

// QueueDetails is a single interface that extends the definition
// given by QueueItem, by appending the three above interfaces to it.
export interface QueueDetails extends QueueItem {
    title_info: QueueTitleInfo
    omdb_info: QueueOmdbInfo
    trouble: QueueTroubleDetails
}

export interface Movie {
    //TODO
}

export interface ItemUpdate {
    item: QueueDetails
    item_position: number
    item_id: number
}

export interface FfmpegUpdate {
    item_id: number
    ffmpeg_instances: CommanderTask[]
}

// The ContentManager class is available for use by components
// who wish to keep track of the servers queue state. Generally
// speaking, the class should only be instantiated once - however
// it's perfecrtly capable of being instantiated multiple times.
class ContentManager {
    private _items: QueueItem[]
    private _details: Map<number, QueueDetails> = new Map()
    private _profiles: TranscodeProfile[] = []
    private _ffmpeg: Map<number, CommanderTask[]> = new Map()
    private _movies: Movie[] = []

    private bootstrapped = false

    queueOrderManager = new QueueOrderManager(
        this.requestQueueIndex.bind(this)
    )

    knownMovies: Writable<Movie[]>
    // movieDetails: Writable<<>>

    constructor() {
        dataStream.subscribe((data: SocketData) => {
            if (!this.bootstrapped) {
                console.warn("ContentManager received a WS update but has not bootstrapped yet! Ignoring update.")
                return
            }

            if (data.type == SocketMessageType.UPDATE)
                this.handleUpdate(data)
        })

        socketStream.subscribe((data: SocketStreamPacket) => {
            if (data.type == SocketPacketType.OPEN && !this.bootstrapped)
                this.bootstrap()
        })

        this.knownMovies = writable(this._movies)
    }

    // bootstrap initializes this instance of the content manager
    // by performing requests to Thea over the websocket
    // to gather the initial state (which can then be
    // updated via websocket updates from this point forth).
    bootstrap() {
        itemIndex.subscribe(items => {
            if (items === undefined) return
            this._items = items
            this.hydrateDetails()
        });

        itemDetails.subscribe((items) => {
            if (items === undefined) return

            console.debug("itemDetails change:", items)
            this._details = items
        })

        itemFfmpegInstances.subscribe((instances) => {
            if (instances === undefined) return

            console.debug("ffmpegInstances change:", instances)
            this._ffmpeg = instances
        })

        ffmpegProfiles.subscribe((profiles) => {
            if (profiles === undefined) return

            console.debug("serverProfiles change:", profiles)
            this._profiles = profiles
        })

        this.requestMovies()
        this.requestQueueIndex()
        this.requestTranscoderProfiles()
        this.bootstrapped = true
    }

    // requestMovies will query the database for a list of known
    // movies that have completed their transcoding. This selection
    // can then be filtered by the client in order to perform filtering,
    // searching, etc...
    requestMovies() {
        if (true) { return }
        const handleReply = (response: SocketData): boolean => {
            if (response.type == SocketMessageType.RESPONSE) {
                itemIndex.set(response.arguments.payload.items)
            } else {
                console.warn("[QueueManager] Invalid reply while fetching queue index.", response)
            }

            return false;
        }

        commander.sendMessage({
            title: "MOVIE_INDEX",
            type: SocketMessageType.COMMAND
        }, handleReply)
    }

    // hydrateDetails is a method that is called automatically
    // when the itemIndex writable store is updated. This method
    // will scan the effective index for missing, or out of date
    // information.
    hydrateDetails() {
        // Find any items that we're missing details for
        const index = this.queueOrderManager.getCurrentQueueIndex()
        const missingItems = index.filter((item) => !this._details.has(item.id))
        missingItems.forEach((item) => this.requestDetails(item.id))

        // Find invalid details (details that no longer have a coresponding entry in the index)
        const invalidDetails = []
        this._details.forEach((item, key) => {
            if (index.findIndex((i) => item.id == i.id) < 0) {
                // Item no longer exists in new details, this entry must be removed
                invalidDetails.push(key)
            }
        })

        // Remove invalid details from the _details
        invalidDetails.forEach((key) => this._details.delete(key))
        itemDetails.set(this._details)
    }

    // requestIndex will query the server for the index of items (the queue)
    // that is currently known to the server. This client is responsible for
    // identifying new items and hydrating their details (see hydrateDetails).
    requestQueueIndex() {
        const handleReply = (response: SocketData): boolean => {
            if (response.type == SocketMessageType.RESPONSE) {
                this.queueOrderManager.replaceQueueList(response.arguments.payload)
            } else {
                console.warn("[QueueManager] Invalid reply while fetching queue index.", response)
            }

            return false;
        }

        commander.sendMessage({
            title: "QUEUE_INDEX",
            type: SocketMessageType.COMMAND
        }, handleReply)
    }

    // requestDetails will lodge a query to the server for enhanced details for
    // a particular QueueItem (specified by the provided itemId).
    requestDetails(itemId: number) {
        const handleReply = (response: SocketData): boolean => {
            if (response.type == SocketMessageType.RESPONSE) {
                this._details.set(itemId, response.arguments.payload)
                itemDetails.set(this._details)

                this._ffmpeg.set(itemId, response.arguments.instances)
                itemFfmpegInstances.set(this._ffmpeg)
            } else {
                console.warn("[QueueManager] Invalid reply while fetching queue details.", response)
            }

            return false;
        }

        commander.sendMessage({
            title: "QUEUE_DETAILS",
            type: SocketMessageType.COMMAND,
            arguments: {
                id: itemId,
            }
        }, handleReply)
    }

    // requestProfiles will query the server for the list of profiles
    // the server is matching queue items against when handling their
    // transcode. These profiles can be modified by the client by sending
    // messages to the server via the Commander.
    requestTranscoderProfiles() {
        const handleReply = (response: SocketData): boolean => {
            if (response.type == SocketMessageType.RESPONSE) {
                ffmpegProfiles.set(response.arguments.payload)
            } else {
                console.warn("[QueueManager] Invalid reply while fetching profile index.", response)
            }

            return false;
        }

        commander.sendMessage({
            title: "PROFILE_INDEX",
            type: SocketMessageType.COMMAND
        }, handleReply)
    }

    // handleUpdate is called by the ContentManager whenever a packet
    // is received that indicates an UPDATE has occured on the server.
    // The update's type is checked and depending on *what* has updated
    // on the server, the client will query the server for new information
    private handleUpdate(data: SocketData) {
        const update = data.arguments.context
        if (update.UpdateType == 0) {
            const itemUpdate = update.Payload as ItemUpdate
            const idx = this._items.findIndex(item => item.id == itemUpdate.item_id)
            if (itemUpdate.item_position < 0 || !itemUpdate.item) {
                // Item has been removed from queue! Find the item
                // in the queue with the ID that matches the one removed
                // and pull it from the list
                if (idx < 0) {
                    console.warn("Failed to find item inside of list for removal. Forcing refresh!")
                    this.requestQueueIndex()

                    return
                }

                this.queueOrderManager.removeItem(itemUpdate.item_id)
            } else if (idx != itemUpdate.item_position) {
                // The position for this item has changed.. likely due to a item promotion.
                // Update the order of the queue                
                if (!this.queueOrderManager.moveItem(itemUpdate.item_id, itemUpdate.item_position)) {
                    this.requestQueueIndex()
                }
            } else {
                if (idx < 0) {
                    // New item
                    this.queueOrderManager.insertItem(itemUpdate.item)
                } else {
                    // An existing item has had an in-place update.
                    this._details.set(itemUpdate.item_id, itemUpdate.item)
                    itemDetails.set(this._details)
                }

            }
        } else if (update.UpdateType == 1) {
            console.log("Queue update received from server - refetching item indexes")
            this.requestQueueIndex()
        } else if (update.UpdateType == 2) {
            console.log("Profile update received from server - fetching profile information")
            this.requestTranscoderProfiles()
        } else if (update.UpdateType == 3) {
            console.log("FFmpeg update receives from server")

            const ffmpegUpdate = update.Payload as FfmpegUpdate
            if (!this._details.has(ffmpegUpdate.item_id)) {
                // Hm, we received an ffmpeg update for an item we don't even know aboout...
                console.warn("Received ffmpeg update for unknown item. Forcing refresh!")
                this.requestQueueIndex()

                return
            }

            this._ffmpeg.set(ffmpegUpdate.item_id, ffmpegUpdate.ffmpeg_instances)
            itemFfmpegInstances.set(this._ffmpeg)
        }
    }
}

export const contentManager = new ContentManager();