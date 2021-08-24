<script lang="ts">
import type { QueueDetails } from "../queue";
import ellipsisHtml from '../assets/html/ellipsis.html';
import workingHtml from '../assets/html/dual-ring.html';
import errHtml from '../assets/err.svg';
import checkHtml from '../assets/check-mark.svg';
import pendingHtml from '../assets/pending.svg';

export let details:QueueDetails;
export let stageIndex:number;
export let drawLines:boolean = false;

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

// getCheckClass is a dynamic binding that is used to
// get the HTML 'class' that must be applied to each
// 'check' icon inbetween each pipeline stage in the Overview.
// This class is used to adjust the color and connecting lines
// to better reflect the situation (e.g. red with no line
// after the icon to indicate an error)
$:getCheckClass = function():string {
    if(stageIndex < details.stage) {
        return 'complete'
    } else if(stageIndex == details.stage) {
        return details.trouble ? 'trouble' : (details.status == 0 ? 'pending' : 'working')
    } else {
        return 'queued'
    }
}
</script>

<style lang="scss">
@use "../styles/stageIcon.scss";
</style>

<div class="check {getCheckClass()}" class:draw-lines={drawLines}>
    {@html getStageIcon()}
</div>
