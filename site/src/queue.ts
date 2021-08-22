import { Writable, writable } from "svelte/store"
import { commander, dataStream } from "./commander"
import { SocketData, SocketMessageType } from "./store"

export enum QueueStatus {
    PENDING,
    PROCESSING,
    COMPLETED,
    NEEDS_RESOLVING,
    CANCELLING,
    CANCELLED,
}

export enum QueueStage {
    IMPORT,
    TITLE,
    OMDB,
    FFMPEG,
    DB,
    FINISH
}

export enum QueueTroubleType {
	TITLE_FAILURE,
	OMDB_NO_RESULT_FAILURE,
	OMDB_MULTIPLE_RESULT_FAILURE,
	OMDB_REQUEST_FAILURE,
	FFMPEG_FAILURE,
}

export interface QueueItem {
    id: number
    name: string
    stage: QueueStage
    status: QueueStatus
    statusLine: string
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
    omdb_info:  QueueOmdbInfo
    trouble: QueueTroubleDetails
}
// The QueueManager class is available for use by components
// who wish to keep track of the servers queue state. Generally
// speaking, the class should only be instantiated once - however
// it's perfecrtly capable of being instantiated multiple times.
export class QueueManager {
    private _items: QueueItem[] = []
    private _details: QueueDetails[] = []

    itemIndex: Writable<QueueItem[]>
    itemDetails: Writable<QueueDetails[]>

    constructor() {
        dataStream.subscribe((data: SocketData) => {
            if(data.type == SocketMessageType.UPDATE)
                this.handleUpdate(data)
        })

        this.itemIndex = writable(this._items)
        this.itemDetails = writable(this._details)

        this.itemIndex.subscribe((items) => {
            console.log("itemIndex change:", items)
            this._items = items
            this.hydrateDetails(items)
        })

        this.itemDetails.subscribe((items) => {
            console.log("itemDetails change:", items)
            this._details = items
        })

        this.requestIndex()
    }

    // hydrateDetails is a method that is called automatically
    // when the itemIndex writable store is updated. This method
    // will scan the itemDetails store for missing, or out of date
    // information.
    hydrateDetails(newData: QueueItem[]) {
        // Find any items that we're missing details for, or that no longer exist
        const purgedDetails = this._details.filter((item) => newData.findIndex((i) => i.id == item.id) > -1)
        const missingItems = newData.filter((item) => purgedDetails.findIndex((i) => i.id == item.id) < 0)

        missingItems.forEach((item) => this.requestDetails(item.id))
        this.itemDetails.set(purgedDetails)
    }

    requestIndex() {
        const handleReply = (response: SocketData): boolean => {
            if(response.type == SocketMessageType.RESPONSE) {
                this.itemIndex.set(response.arguments.payload.items)
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

    requestDetails(itemId: number) {
        const handleReply = (response: SocketData): boolean => {
            if(response.type == SocketMessageType.RESPONSE) {
                this._details[itemId] = response.arguments.payload
                this.itemDetails.set(this._details)
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

    private handleUpdate(data: SocketData) {
        const update = data.arguments.context
        const newItem = update.QueueItem as QueueDetails

        const idx = this._items.findIndex(item => item.id == update.ItemId)
        if( update.ItemPosition < 0 || !newItem ) {
            // Item has been removed from queue! Find the item
            // in the queue with the ID that matches the one removed
            // and pull it from the list
            if(idx < 0) {
                console.warn("Failed to find item inside of list for removal. Forcing refresh!")
                this.requestIndex()

                return
            }

            this._items.splice(idx, 1)
            // Svelte reactivity requires re-assignment of an array if it's modified using
            // a mutating method like 'splice'
            this.itemIndex.set(this._items)
        } else if(idx != update.ItemPosition) {
            // The position for this item has changed.. likely due to a item promotion.
            // Update the order of the queue - to do this we should
            // simply re-query the server for an up-to-date queue index.
            this.requestIndex()
        } else {
            if(idx < 0) {
                // New item
                this._items.push(newItem)
                this.itemIndex.set(this._items)
            } else {
                // An existing item has had an in-place update.
                this._details[newItem.id] = newItem
                this.itemDetails.set(this._details)
            }

        }
    }
}
