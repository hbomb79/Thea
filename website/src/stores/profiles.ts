import { Writable, writable } from "svelte/store";

import type { TranscodeProfile } from "../queue";

export const ffmpegProfiles: Writable<TranscodeProfile[]> = writable([]);