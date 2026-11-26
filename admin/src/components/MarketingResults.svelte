<script>
 export let result=null;
 const counts={total:'Total visits',trusted:'Trusted visits',tracked:'Tracked visits',wishlists:'Wishlists',purchases:'Purchases',activations:'Activations'};
</script>
{#if result?.observed}
 <dl>{#each Object.entries(counts) as [key,label]}<div><dt>{label}</dt><dd>{result[key]?.toLocaleString()??'—'}</dd></div>{/each}</dl>
 <p>Reported totals · {result.observed} of {result.expected} placement ranges matched. A dash means no value was reported.</p>
{:else}
 <p>No matching results for recorded posts yet.</p>
{/if}
<style>
 dl{display:flex;flex-wrap:wrap;gap:.5rem;margin:.5rem 0;text-align:left}
 dl div{min-width:5.5rem;padding:.35rem .5rem;background:var(--paper-warm);border-radius:4px}
 dt{color:var(--text-muted);font-size:.78rem}
 dd{font-size:1.05rem;font-weight:700;margin:0}
 p{text-align:left;margin:.5rem 0;color:var(--text-muted);font-size:.86rem}
</style>
