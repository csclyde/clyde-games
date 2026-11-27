<script>
 import {coverageHistory} from '../marketing-coverage.mjs';
 export let target;
 const number=new Intl.NumberFormat();
 $: rows=coverageHistory(target);
</script>

<section class="history" aria-labelledby="coverage-history-title">
 <h3 id="coverage-history-title">Coverage history</h3>
 {#if rows.length}
  <table aria-labelledby="coverage-history-title">
   <thead><tr><th scope="col">Title</th><th scope="col">Published</th><th scope="col">Engagement</th></tr></thead>
   <tbody>
    {#each rows as row (row.href)}
     <tr>
      <td><a href={row.href} rel="noreferrer" target="_blank">{row.title}</a></td>
      <td>{#if row.date}<time datetime={row.date}>{row.date}</time>{:else}Date unavailable{/if}</td>
      <td>
       {#if row.metric}
        <span class="value">{number.format(row.metric.value)} {row.metric.label}</span>
        {#if row.metric.shared}<small>Whole item</small>{/if}
        {#if row.metric.observedOn}<small>As of <time datetime={row.metric.observedOn}>{row.metric.observedOn}</time></small>{/if}
       {:else}Not recorded{/if}
      </td>
     </tr>
    {/each}
   </tbody>
  </table>
 {:else}
  <p>No verified coverage recorded yet.</p>
 {/if}
</section>

<style>
 .history{border-top:1px solid var(--line);margin-top:.75rem;padding-top:.6rem}
 h3,p{margin:.25rem 0 .5rem;text-align:left}
 h3{font-size:.94rem}
 p,td,th{font-size:.85rem}
 p,small{color:var(--text-muted)}
 table{width:100%;border-collapse:collapse;text-align:left;table-layout:fixed}
 th,td{padding:.4rem .45rem;vertical-align:top;border-bottom:1px solid var(--line);overflow-wrap:anywhere}
 th{background:var(--paper-warm);font-weight:650}
 th:first-child{width:50%}
 td:nth-child(2),td:nth-child(3){font-variant-numeric:tabular-nums}
 a{color:var(--forest);text-decoration:underline}
 .value{font-weight:650}
 small{display:block}
 @media(max-width:500px){th:first-child{width:42%}th,td{padding:.35rem .24rem;font-size:.78rem}}
</style>
