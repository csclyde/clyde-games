<script>
 import {creatorOutreach} from '../marketing-outreach.mjs';
 export let target;
 $: status=creatorOutreach(target);
</script>

<section class="outreach" aria-label="Creator outreach">
 <dl>
  <div><dt>Outreach</dt><dd>{status.contact}</dd></div>
  <div><dt>Response</dt><dd class:replied={status.responded}>{status.response}{#if status.date} <time datetime={status.date}>{status.date}</time>{/if}</dd></div>
 </dl>
 {#if status.url}<a href={status.url} target="_blank" rel="noreferrer">{status.linkText}</a>{/if}
</section>
<style>
 .outreach{border:1px solid var(--line);border-radius:5px;background:var(--paper-warm);display:flex;flex-wrap:wrap;align-items:center;justify-content:space-between;gap:.4rem .9rem;margin:.6rem 0;padding:.55rem .65rem;font-size:.83rem}
 dl{margin:0;display:flex;flex-wrap:wrap;gap:.4rem 1.2rem}
 dt{color:var(--text-muted)}
 dd{margin:0;font-weight:650}
 .replied,a{color:var(--forest)}
 time{font-weight:400;color:var(--text-muted);margin-left:.25rem}
 a{text-decoration:underline}
</style>
