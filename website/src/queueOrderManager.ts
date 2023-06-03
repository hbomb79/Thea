
import type { QueueItem } from "./queue";
import { commander } from "./commander";
import { SocketMessageType } from "./stores/socket";
import type { SocketData } from "./stores/socket";
import { itemIndex, queueState } from "./stores/queue";

export enum QueueState {
    SYNCED,
    REORDERING,
    SYNCING,
    FAILURE,
}

/**
 * Thea allows users to reorder the queue items in the list in a drag-and-drop manner. This
 * UI handles the reordering by allowing the user to re-order the list purely client-side, before
 * 'commiting' to the re-order, which sends the reorder request to Thea.
 *
 * This class encapsulates that logic, and manages the current state of the queue index as well as the
 * request/response logic to Thea.
 *
 * There's potential for problems with this approach, especially if other components in Thea are
 * accessing the queue index directly via the global stores. This solution is somewhat temporary.
 */
export class QueueOrderManager {
    private _currentIndex: QueueItem[] = [];
    private _currentState: QueueState = QueueState.SYNCED;
    private stale: boolean = false;

    private resetter: () => void;

    constructor(reset: () => void) {
        this.resetter = reset;

        queueState.subscribe(state => this._currentState = state)
        itemIndex.subscribe(index => this._currentIndex = index)
    }

    handleReorder(newIndex: QueueItem[]) {
        itemIndex.set(newIndex);
        queueState.set(QueueState.REORDERING)
    }

    /**
     * Commits an in-progress queue reorder to Thea.
     * 
     * If the manager was marked as stale (e.g. due to a call to replaceQueueList
     * being made during the reordering) the manager will completely re-sync it's
     * queue list from Thea using the resetter.
     * Likewise if the queue reorder is rejected by Thea.
     */
    commitReorder() {
        if (this._currentState != QueueState.REORDERING) return

        queueState.set(QueueState.SYNCING);
        commander.sendMessage(
            {
                title: "QUEUE_REORDER",
                type: SocketMessageType.COMMAND,
                arguments: {
                    index: this._currentIndex.map((item) => item.id),
                },
            },
            (replyData: SocketData): boolean => {
                if (replyData.type == SocketMessageType.ERR_RESPONSE) {
                    console.warn("Queue reordering failed - requesting up-to-date index from server", replyData);
                    this.stale = true;
                }

                this._currentState = QueueState.SYNCED;
                if (this.stale) this.resetter()

                return false;
            }
        );
    }

    /**
     * Removes the item from the effective queue list, ignoring on-going
     * queue re-orders.
     * 
     * @param index The items ID to remove
     */
    removeItem(itemID: number) {
        itemIndex.update(items => items.filter((item) => item.id != itemID))
    }

    /**
     * Inserts the item provided in to the effective queue list, ignoring on-going queue re-orders.
     * 
     * @param newItem The new item to insert
     */
    insertItem(newItem: QueueItem) {
        itemIndex.update(items => {
            items.push(newItem)
            return items
        })
    }

    /**
     * Moves the item specified by itemID to the position specified.
     * 
     * @param itemID The item ID to move
     * @param newPosition The new position in the queue list that this item should occupy
     * @return A boolean indiciating success (false on failure)
     */
    moveItem(itemID: number, newPosition: number): boolean {
        const itemIdx = this._currentIndex.findIndex((item) => item.id == itemID)
        if (itemIdx == -1) return false;

        itemIndex.update(current => {
            const deleted = current.splice(itemIdx, 1)
            current.splice(newPosition, 0, ...deleted)
            return current
        })
    }

    /**
     * Replaces the existing queue list with the new index provided. This method should only be used in
     * cases where we've detected that the client has fallen out of sync with Thea.
     * 
     * If the queue list is currently being re-ordered by the user, then this call will mark the manager as
     * stale. When the user finishes their re-order 
     * 
     * @param newIndex The new queue list to replace the existing one with
     */
    replaceQueueList(newIndex: QueueItem[]) {
        if (this._currentState == QueueState.REORDERING) {
            console.warn("Queue index change from server was IGNORED as queue is being reordered!");
            this.stale = true;
            return
        }

        queueState.set(QueueState.SYNCED);
        itemIndex.set(newIndex)
        this.stale = false;
    }

    getCurrentQueueIndex(): QueueItem[] {
        return this._currentIndex
    }
}
