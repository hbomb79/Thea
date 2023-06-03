import { Writable, writable } from "svelte/store"
import type { QueueItem, QueueDetails, CommanderTask } from "../queue"
import { QueueState } from "../queueOrderManager"


export const itemIndex: Writable<QueueItem[]> = writable([])
export const itemDetails: Writable<Map<number, QueueDetails>> = writable(new Map)
export const itemFfmpegInstances: Writable<Map<number, CommanderTask[]>> = writable(new Map)
export const queueState: Writable<QueueState> = writable(QueueState.SYNCED)