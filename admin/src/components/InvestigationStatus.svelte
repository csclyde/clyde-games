<script>
 import {summarizeInvestigation} from '../investigation-status.mjs';
 export let statuses=[];
 export let inline=false;
 $: summary=summarizeInvestigation(statuses);
 $: label=inline?summary.label.replace(' · ready for review',' / ready for review').replace('Investigation status loading','Loading status').replace('Investigation status unavailable','Status unavailable'):summary.label;
</script>
<span class="investigation-status" class:inline class:ready={summary.state==='ready_for_review'} class:blocked={summary.state==='blocked'} class:investigating={summary.state==='investigating'} class:needs-confirmation={summary.state==='needs_confirmation'} class:unreviewed={summary.state==='unreviewed'} class:loading={summary.state==='loading'} class:unavailable={summary.state==='unavailable'} aria-label="Investigation status">
 <b>{label}</b>{#if summary.release}<span>{summary.release==='local'?'Local · unreleased':summary.release==='released'?'Released to players':'Deployed'}</span>{/if}
</span>
<style>.investigation-status{display:inline-flex;align-items:baseline;flex-wrap:wrap;gap:5px 9px;margin:5px 0;padding:4px 7px;border:1px solid var(--line);border-radius:4px;background:var(--paper-warm);font-size:.72rem;text-align:left;max-width:100%;box-sizing:border-box;overflow-wrap:anywhere;color:var(--text-muted)}b{font-weight:700}span span{font-size:.68rem;color:var(--text-soft)}.ready{border-color:var(--forest);color:var(--forest)}.blocked,.unavailable{color:var(--rust)}.inline{display:inline;margin:0;padding:0;border:0;border-radius:0;background:transparent;vertical-align:baseline}.inline b{font-weight:600}.inline>span::before{content:' · '}.inline.investigating{color:var(--slate)}.inline.needs-confirmation{color:#83601f}.inline.unreviewed,.inline.loading{color:var(--text-soft)}</style>
