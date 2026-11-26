<script>
 import {impactLabel,impactLevel} from '../marketing-targets.mjs';
 export let target;
</script>

<span class={'badge impact level-'+impactLevel(target)}><span class="screen-reader">Impact: </span>{impactLabel(target)}</span>
{#if target.new_status?.is_new}
 <span class="badge new" title={target.new_status.label}>New<span class="screen-reader"> · {target.new_status.label}</span></span>
{/if}

<style>
 .badge{display:inline-block;flex:none;white-space:nowrap;border-radius:999px;padding:.08rem .4rem;font-size:.69rem;font-weight:700}
 .impact{background:#e9e9e7;color:#484946}
 .level-1{background:#f8dede;color:#831f28}
 .level-2{background:#fff0bf;color:#674900}
 .level-3{background:#dcefe0;color:#235832}
 .new{background:#e4edf9;color:#244d80}
 .screen-reader{position:absolute;width:1px;height:1px;overflow:hidden;clip-path:inset(50%);white-space:nowrap}
</style>
