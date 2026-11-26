<script>
 import {createEventDispatcher,onDestroy} from 'svelte';
 import Panel from './Panel.svelte';
 export let selectedProject='all';export let refreshToken=0;
 const dispatch=createEventDispatcher();let days=30,loadedScope=null,loadedDays=null,loadedToken=null,sequence=0,controller,data=null,loading=true,error='';
 $: if(selectedProject!==loadedScope||days!==loadedDays||refreshToken!==loadedToken){loadedScope=selectedProject;loadedDays=days;loadedToken=refreshToken;load()}
 async function load(){const request=++sequence;controller?.abort();controller=new AbortController();loading=true;error='';data=null;dispatch('loaded',null);const scope=selectedProject,windowDays=days;
  try{const r=await fetch(`/api/crash/health?project=${encodeURIComponent(scope)}&days=${windowDays}`,{signal:controller.signal,cache:'no-store'});const body=await r.json();if(!r.ok)throw new Error(body.error||'Crash health unavailable');if(!body.complete||body.project!==scope||!Array.isArray(body.daily)||!Array.isArray(body.builds)||!Array.isArray(body.groups)||!Array.isArray(body.families))throw new Error('Complete crash health data unavailable');if(request!==sequence)return;data=body;dispatch('loaded',body)}catch(e){if(request===sequence&&e.name!=='AbortError')error=e.message}finally{if(request===sequence)loading=false}}
 onDestroy(()=>{sequence++;controller?.abort()});
 const number=n=>(n||0).toLocaleString();
 function date(value){return value?new Date(value).toLocaleString(undefined,{month:'short',day:'numeric',year:'numeric',hour:'numeric',minute:'2-digit'}):'No detailed receipt yet'}
 $: maxDaily=Math.max(1,...(data?.daily||[]).map(d=>d.reports));
 $: maxBuild=Math.max(1,...(data?.builds||[]).map(b=>b.reports));
 $: reportDays=data?.daily||[];
 $: firstDay=data?.first_received?.slice(0,10);
 $: unknownDays=firstDay?reportDays.filter(d=>d.day<firstDay).length:reportDays.length;
</script>
<details class="crash-health crash-analysis" aria-busy={loading}><summary class="analysis-toggle"><strong>Analysis</strong><span>Trends, builds and reporting IDs</span>{#if error}<small>Unavailable</small>{/if}</summary><div class="analysis-content">
 <Panel inset={false} title="Crash health" subtitle="Detailed reports by server receipt time. Closed reports remain in these counts.">
  <div slot="actions"><label class="window-label">Receipt window<select aria-label="Crash receipt window" bind:value={days}><option value={7}>Last 7 days</option><option value={30}>Last 30 days</option><option value={90}>Last 90 days</option></select></label></div>
  {#if loading}<p class="notice" role="status">Loading crash health…</p>
  {:else if error}<p class="notice error" role="alert">{error}</p>
  {:else if data}
   <div class="metrics">
    <article><span>Detailed reports</span><strong>{number(data.reports)}</strong><small>Received in the last {days} days</small></article>
    <article><span>Reporting IDs</span><strong>{number(data.reporting_ids)}</strong><small>Distinct client PIDs · {number(data.missing_pid_reports)} reports without an ID</small></article>
    <article><span>Open issue groups</span><strong>{number(data.open_groups)}</strong><small>Exact identities · across stored history</small></article>
    <article class:returned={data.returned_open_groups>0}><span>Returned open groups</span><strong>{number(data.returned_open_groups)}</strong><small>{number(data.returned_reports)} recent reports · {number(data.returned_reporting_ids)} reporting IDs</small></article>
   </div>
   <p class="definition">Distinct client PIDs, counted separately per project. These are not verified people; active-installation and session totals are unavailable. Returned groups received new reports after their latest recorded closure.</p>
  {/if}
 </Panel>
 {#if data}
 <div class="health-panels">
  <Panel inset={false} title="Receipt activity" subtitle="Stored detailed receipts · UTC days">
   {#if data.reports===0}<p class="notice">No detailed reports received in this window.</p>{/if}
   <svg class="activity" viewBox="0 0 620 150" role="img" aria-label="Daily detailed crash reports by UTC receipt date">
    <defs><pattern id="crash-history-gap" width="6" height="6" patternUnits="userSpaceOnUse"><path d="M0,6 L6,0" stroke="var(--line)" stroke-width="1"/></pattern></defs>
    {#if unknownDays}<rect class="no-history" x="10" y="15" width={unknownDays*600/reportDays.length} height="115"><title>No detailed records before the first stored receipt; legacy history is unavailable on this timeline</title></rect>{/if}
    <line x1="10" x2="610" y1="130" y2="130" class="baseline"/>
    {#each reportDays as day,index}<rect x={10+index*600/reportDays.length} y={130-day.reports/maxDaily*115} width={Math.max(1,600/reportDays.length-3)} height={day.reports/maxDaily*115} rx="2"><title>{day.day}: {day.reports} detailed reports, {day.reporting_ids} reporting IDs</title></rect>{/each}
   </svg>
   <div class="chart-range"><span>{reportDays[0]?.day||''}</span><span>{reportDays[reportDays.length-1]?.day||''}</span></div>
   {#if unknownDays}<p class="definition">{data.first_received ? `First stored detailed receipt: ${date(data.first_received)}. Earlier days have no detailed records.` : 'No detailed receipt history is available.'}</p>{/if}
   <details class="daily-details"><summary>View daily counts</summary><div class="table-scroll"><table><thead><tr><th>UTC date</th><th>Reports</th><th>Reporting IDs</th></tr></thead><tbody>{#each reportDays as day}<tr><td>{day.day}</td><td>{day.reports}</td><td>{day.reporting_ids}</td></tr>{/each}</tbody></table></div></details>
  </Panel>
  <Panel inset={false} title="Recorded history" subtitle="Legacy aggregates stay separate from detailed receipts.">
   <dl class="history"><div><dt>First detailed receipt</dt><dd>{date(data.first_received)}</dd></div><div><dt>Latest detailed receipt</dt><dd>{date(data.last_received)}</dd></div><div><dt>Detailed records · all time</dt><dd>{number(data.detailed_all_time)}</dd></div><div><dt>Legacy aggregate records</dt><dd>{number(data.legacy_rows)}</dd></div><div><dt>Reported legacy occurrences</dt><dd>{number(data.legacy_occurrences)}</dd></div></dl>
   <p class="definition">Detailed history begins at the first recorded receipt. Legacy totals have incomplete timestamps, builds and installation history. They do not contribute to the chart, build comparisons or reporting-ID counts.{#if data.untimed_detailed} {number(data.untimed_detailed)} detailed records without a valid receipt time are also excluded from timed counts.{/if}</p>
  </Panel>
 </div>
 <Panel inset={false} title="Build comparison" subtitle={`Detailed reports received in the last ${days} days. Counts reflect reporting volume, not crash rates.`}>
  {#if !data.builds.length}<p class="notice">No detailed build observations in this window.</p>{:else}
  <div class="table-scroll"><table class="build-comparison"><thead><tr><th>Build / project</th><th>Reports</th><th>Reporting IDs</th><th>Issue groups</th><th>Open reports</th><th>Returned reports</th></tr></thead><tbody>
   {#each data.builds as build}<tr><td><button class="build-link" on:click={()=>dispatch('filter',{project:build.project,build:build.build})}>{build.build||'Unknown build'}</button><small>{build.project||'Untagged legacy client'}</small></td><td><div class="build-volume"><span class="volume" style={`width:${build.reports/maxBuild*100}%`}></span><b>{number(build.reports)}</b></div></td><td>{number(build.reporting_ids)}{#if build.missing_pid_reports}<small>{number(build.missing_pid_reports)} reports without ID</small>{/if}</td><td>{number(build.groups)}</td><td>{number(build.open_reports)}</td><td>{number(build.returned_reports)}</td></tr>{/each}
  </tbody></table></div><p class="definition">Select a build to filter the open issue queue. No session totals or crash-free rate are inferred from crash submissions.</p>
  {/if}
 </Panel>
 {/if}
</div></details>
<style>
 .crash-health{margin:18px 0 0;min-width:0;border:1px solid var(--line);border-radius:var(--radius);background:var(--surface)}.analysis-toggle{padding:12px 16px;cursor:pointer;text-align:left;font-size:.84rem}.analysis-toggle span{color:var(--text-soft);font-size:.75rem;margin-left:12px}.analysis-toggle small{color:var(--rust);margin-left:10px}.analysis-content{display:grid;gap:12px;padding:0 12px 12px;min-width:0}.window-label{display:flex;align-items:center;gap:8px;font-size:.76rem;font-weight:700;color:var(--text-soft)}select{font:inherit;color:var(--text);background:var(--surface);border:1px solid var(--line-strong);border-radius:4px;padding:7px;max-width:100%}.metrics{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:14px}.metrics article{text-align:left;padding:10px 0 8px;border-top:2px solid var(--line)}.metrics span,.metrics small{display:block;text-align:left;font-size:.72rem;line-height:1.5;color:var(--text-soft)}.metrics span{font-weight:800;text-transform:uppercase;letter-spacing:.04em}.metrics strong{display:block;font-family:var(--font-mono);font-size:2rem;line-height:1.35;letter-spacing:-.06em;color:var(--forest)}.metrics article.returned{border-color:var(--rust)}.metrics article.returned strong{color:var(--rust)}.definition,.notice{font-size:.75rem;line-height:1.65;color:var(--text-soft);margin:12px 0 0;text-align:left}.error{color:var(--rust)}.health-panels{display:grid;grid-template-columns:1.3fr 1fr;gap:12px;min-width:0}.activity{display:block;width:100%;height:150px;margin-top:12px;overflow:visible}.activity rect{fill:var(--forest)}.activity .no-history{fill:url(#crash-history-gap)}.baseline{stroke:var(--line);stroke-width:1}.chart-range{display:flex;justify-content:space-between;font: .68rem var(--font-mono);color:var(--text-soft)}.daily-details{font-size:.76rem;margin-top:16px}.daily-details summary{cursor:pointer;font-weight:700}.table-scroll{overflow-x:auto;max-width:100%;min-width:0}table{width:100%;border-collapse:collapse;font-size:.78rem;text-align:left}th{font-size:.66rem;text-transform:uppercase;letter-spacing:.04em;color:var(--text-soft);white-space:nowrap}td,th{padding:11px 10px;border-bottom:1px solid var(--line);vertical-align:middle}td:first-child,th:first-child{padding-left:0}td small{display:block;text-align:left;color:var(--text-soft);font-size:.68rem;margin-top:3px}td{font-variant-numeric:tabular-nums}.build-comparison{min-width:690px}.build-link{background:transparent;border:0;padding:0;text-align:left;font: .76rem var(--font-mono);font-weight:700;color:var(--forest);text-decoration:underline;cursor:pointer}.build-volume{position:relative;width:110px;min-height:24px;display:flex;align-items:center}.build-volume b{position:relative;font: .8rem var(--font-mono);padding:2px 7px}.volume{position:absolute;left:0;top:0;height:100%;background:var(--forest);opacity:.13;border-radius:3px}.history{margin:0}.history div{display:flex;align-items:baseline;justify-content:space-between;gap:12px;padding:8px 0;border-bottom:1px solid var(--line)}dt{font-size:.73rem;color:var(--text-soft)}dd{margin:0;font-size:.76rem;text-align:right;font-weight:600;overflow-wrap:anywhere}@media(max-width:900px){.metrics{grid-template-columns:repeat(2,minmax(0,1fr))}.health-panels{grid-template-columns:minmax(0,1fr)}}@media(max-width:420px){.metrics{gap:8px}.metrics strong{font-size:1.7rem}.window-label{flex-wrap:wrap}.history div{display:block}.history dd{text-align:left;margin-top:3px}}
</style>
