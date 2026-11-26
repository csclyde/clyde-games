<script>
 export let target;
 const levels=['Unscored','Low','OK','Great'];
 const metrics={youtube_views:'YouTube views',youtube_short_views:'YouTube Shorts views',reddit_score:'Reddit net score'};
 const count=new Intl.NumberFormat();
 const shown=n=>n==null?'Unavailable':count.format(n);
 $: impact=target.impact||{level:0};
 $: items=target.impact_evidence?.items||[];
 $: steam=target.steam_attribution;
</script>

<section aria-label="Target impact">
 <h3>Impact</h3>
 <p><strong>{levels[impact.level]}{impact.level?' · '+impact.level+'/3':''}</strong>
  {#if impact.as_of} · {metrics[impact.metric]} observed {impact.as_of}{/if}
 </p>
 {#if items.length}
  <ul>
   {#each items as item (item.url)}
    <li>
     <a href={item.url} target="_blank" rel="noreferrer">{item.title}</a>
     <p>{metrics[item.metric]}: {shown(item.value)} · {item.observed_on}
      {#if item.comments!=null} · {shown(item.comments)} comments{/if}
      {#if item.scope==='shared'} · Whole item count, excluded from score{/if}
     </p>
     {#if item.note}<p>{item.note}</p>{/if}
    </li>
   {/each}
  </ul>
 {:else}<p>No measured Cursemark observations saved.</p>{/if}
 <details>
  <summary>How reach is scored</summary>
  <p>The strongest measured Cursemark item sets the score. Counts are snapshots on the dates shown. Repeat uploads, cross-posts and overlapping audiences are never added together.</p>
  <dl>
   <div><dt>YouTube video views</dt><dd>OK at 1,000 · Great at 10,000</dd></div>
   <div><dt>YouTube Shorts views</dt><dd>OK at 3,000 · Great at 30,000</dd></div>
   <div><dt>Reddit net score</dt><dd>OK at 10 · Great at 50</dd></div>
  </dl>
  <p>Measured counts below OK are Low. Missing counts stay unscored. Reddit scores can fluctuate.</p>
 </details>
 <h4>Steamworks attribution</h4>
 {#if steam}
  <p>Matched placement reports, {steam.start_date} through {steam.end_date}. Attribution is shown separately from reach.</p>
  <dl>
   <div><dt>Tracked visits</dt><dd>{shown(steam.tracked)}</dd></div>
   <div><dt>Wishlists</dt><dd>{shown(steam.wishlists)}</dd></div>
   <div><dt>Purchases</dt><dd>{shown(steam.purchases)}</dd></div>
   <div><dt>Activations</dt><dd>{shown(steam.activations)}</dd></div>
  </dl>
 {:else}<p>No matching Steamworks results for published placements on this target.</p>{/if}
</section>

<style>
 section{margin-top:.8rem;border-top:1px solid var(--line);padding-top:.45rem}
 h3,h4,p{margin:.35rem 0}
 h3{font-size:.94rem}
 h4{font-size:.89rem;margin-top:.65rem}
 p,dl,details,ul{font-size:.85rem}
 p,dd{color:var(--text-muted)}
 ul{padding-left:1.2rem}
 li{margin:.45rem 0}
 a{color:var(--forest);text-decoration:underline}
 summary{cursor:pointer;color:var(--forest)}
 dl{display:flex;gap:.45rem 1rem;flex-wrap:wrap;margin:.4rem 0}
 dt{font-weight:650}
 dd{margin:0}
</style>
