<script>
 import {crashBuildSeries} from '../crash-builds.mjs';
 export let reports = [];
 $: series = crashBuildSeries(reports);
</script>
<figure class="crash-build-chart" aria-label="Recorded occurrences by build">
 <figcaption>Recorded occurrences by build <span>{series.recorded}</span></figcaption>
 {#if series.builds.length}
  <ol class="build-plot" aria-label="Oldest build first, undated builds last">
   {#each series.builds as build (build.key)}
    <li class:undated={!build.date} data-build={build.build} data-count={build.count} title={`${build.build || 'Build ID unavailable'} · ${build.date ? build.date.day : 'Date unavailable'} · ${build.count} recorded occurrence${build.count === 1 ? '' : 's'}`}>
     <strong>{build.count}</strong>
     <div class="bar-track" aria-hidden="true"><div class="bar" style={`height:${build.count / series.max * 100}%`}></div></div>
     <code>{build.build || 'Build ID unavailable'}</code>
     <small>{build.date ? build.date.day : 'Date unavailable'}</small>
    </li>
   {/each}
  </ol>
 {:else}<p class="empty">No individually recorded occurrences in these open reports.</p>{/if}
 <p class="coverage">Open detailed reports only. Builds with recorded data, oldest to newest; undated builds last. Zero-count builds are not inferred.</p>
 {#if series.legacyRows}<p class="legacy">{series.legacyOccurrences} legacy occurrences in {series.legacyRows} stored record{series.legacyRows === 1 ? '' : 's'} are excluded: their per-occurrence builds are unknown.</p>{/if}
</figure>
<style>
 .crash-build-chart{margin:12px 0 0;min-width:0;border-top:1px solid var(--line);padding:10px 0 0;text-align:left}
 figcaption{font-size:.8rem;font-weight:700;color:var(--text-muted);display:flex;gap:8px;align-items:baseline}figcaption span{font-variant-numeric:tabular-nums;color:var(--text-soft);font-weight:400}
 .build-plot{display:grid;grid-auto-flow:column;grid-auto-columns:minmax(100px,1fr);gap:12px;list-style:none;margin:8px 0;padding:0 0 5px;overflow-x:auto;overscroll-behavior-x:contain}
 li{display:flex;flex-direction:column;align-items:center;min-width:0;text-align:center}strong{font-size:.78rem;font-variant-numeric:tabular-nums}
 .bar-track{height:62px;width:100%;display:flex;align-items:end;justify-content:center;border-bottom:1px solid var(--line)}.bar{width:48px;max-width:65%;min-height:2px;background:var(--forest);border-radius:3px 3px 0 0}.undated .bar{background:var(--olive)}
 code{display:block;font-size:.65rem;line-height:1.35;overflow-wrap:anywhere;white-space:normal;margin-top:5px}small{font-size:.62rem;color:var(--text-soft);margin-top:2px}
 .coverage,.legacy,.empty{font-size:.7rem;line-height:1.45;color:var(--text-soft);text-align:left;margin:5px 0 0}.legacy{color:var(--text-muted)}
</style>
