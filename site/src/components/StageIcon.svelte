<script lang="ts">
import type { QueueDetails } from "../queue";
import { QueueStatus } from "../queue";
import spinnerHtml from '../assets/html/hourglass.html';
import workingHtml from '../assets/html/dual-ring.html';
import troubleSvg from '../assets/err.svg';
import checkSvg from '../assets/check-mark.svg';
import scheduledSvg from '../assets/pending.svg';

export let details:QueueDetails;
export let stageIndex:number;
export let drawLines:boolean = false;

const wrapSpinner = (spinner: string) => `<div class="spinner-wrap">${spinner}</div>`

$:getStageIcon = function():string{
    if(stageIndex < details.stage) {
        return checkSvg
    } else if(stageIndex == details.stage) {
        if(details.status == QueueStatus.NEEDS_RESOLVING) {
            return troubleSvg
        } else if(details.status == QueueStatus.PENDING) {
            return wrapSpinner(spinnerHtml)
        }

        return wrapSpinner(workingHtml)
    } else if(stageIndex > details.stage) {
        return scheduledSvg
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
        return details.trouble && details.status == QueueStatus.NEEDS_RESOLVING ? 'trouble' : (details.status == 0 ? 'pending' : 'working')
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
