<script lang="ts">
import type { QueueDetails } from "./QueueItem.svelte";
import ellipsisHtml from '../assets/html/ellipsis.html';
import workingHtml from '../assets/html/dual-ring.html';
import errHtml from '../assets/err.svg';
import checkHtml from '../assets/check-mark.svg';
import pendingHtml from '../assets/pending.svg';

export let details:QueueDetails;
export let stageIndex:number;

$:getStageIcon = function():string{
    if(stageIndex < details.stage) {
        return checkHtml
    } else if(stageIndex == details.stage) {
        if(details.trouble) {
            return errHtml
        } else if(details.status == 0) {
            return ellipsisHtml
        }

        return workingHtml
    } else if(stageIndex > details.stage) {
        return pendingHtml
    }
}
</script>

{@html getStageIcon()}
