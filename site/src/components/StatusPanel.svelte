<style lang="scss">
@use '../styles/global.scss';
@use 'sass:math';


:global(body.status-panel-open) .panel {
    right: 0;
    opacity: 1;
}

.panel {
    position: fixed;
    opacity: 0;
    z-index: 99;
    top: 0;
    right: - global.$statusPanelWidth;
    bottom: 0;
    width: global.$statusPanelWidth;
    padding-top: global.$navHeight + 2rem;

    background: #ffffff52;

    transition: all global.$statusPanelAnimTime ease-in-out;
    transition-property: opacity, right;

    text-align: center;

    h2 {
        color: #9372ffad;
        font-weight: 400;
    }

    span {
        color: #afb8f5;
    }
}
</style>

<div class="panel">
    <h2>Overview</h2>
    <span>This feature is not yet implemented</span>
</div>
