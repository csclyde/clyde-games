<script>
	import { metricChart, label } from './metricChart';
	export let card;
	export let state = {};
	export let retry = () => {};
	let showData = false;
	let page = 0;
	let sortColumn = '';
	let descending = false;
	let search = '';
	$: result = state.result;
	$: columnSettings = card.settings['table.columns'];
	$: columns = result ? (columnSettings ? columnSettings.filter(c => c.enabled).map(c => c.name).filter(c => result.columns.includes(c)) : result.columns) : [];
	$: filtered = result ? result.rows.filter(row => !search || row.some(value => String(value ?? '').toLowerCase().includes(search.toLowerCase()))) : [];
	$: sorted = sortColumn ? [...filtered].sort((a, b) => {
		const i = result.columns.indexOf(sortColumn);
		const av = a[i], bv = b[i];
		const comparison = av === null ? (bv === null ? 0 : -1) : bv === null ? 1 : av !== '' && bv !== '' && Number.isFinite(Number(av)) && Number.isFinite(Number(bv)) ? Number(av) - Number(bv) : String(av).localeCompare(String(bv));
		return comparison * (descending ? -1 : 1);
	}) : filtered;
	$: pageCount = Math.max(1, Math.ceil(sorted.length / 25));
	$: if (page >= pageCount) page = pageCount - 1;
	$: scalarIndex = result ? Math.max(0, result.columns.indexOf(card.settings['scalar.field'])) : 0;
	function format(value) {
		if (value === null || value === undefined) return '—';
		return typeof value === 'number' ? value.toLocaleString(undefined, { maximumFractionDigits: 3 }) : String(value);
	}
	function sort(column) {
		descending = sortColumn === column ? !descending : false;
		sortColumn = column;
		page = 0;
	}
	function download() {
		const quote = value => `"${String(value ?? '').replace(/"/g, '""')}"`;
		const csv = [result.columns, ...result.rows].map(row => row.map(quote).join(',')).join('\r\n');
		const url = URL.createObjectURL(new Blob([csv], { type: 'text/csv;charset=utf-8' }));
		const link = document.createElement('a');
		link.href = url; link.download = `${card.name}.csv`; link.click();
		setTimeout(() => URL.revokeObjectURL(url), 1000);
	}
</script>

<article class:scalar={card.display === 'scalar'} style={`--span: ${card.width}`}>
	<header>
		<h3>{card.name}</h3>
		{#if result}
			<div class="actions">
				{#if card.display !== 'table'}<button on:click={() => showData = !showData}>{showData ? 'Chart' : 'Data'}</button>{/if}
				<button on:click={download} aria-label={`Download ${card.name} as CSV`}>CSV</button>
			</div>
		{/if}
	</header>
	{#if card.description}<p>{card.description}</p>{/if}
	{#if state.error}
		<p class="error" role="alert">{state.error} <button on:click={retry}>Retry</button></p>
	{:else if !result}
		<p class="state" role="status">Loading…</p>
	{:else if !result.rows.length}
		<p class="state">No data for this project.</p>
	{:else if card.display === 'table' || showData}
		<label class="search">Filter rows <input type="search" bind:value={search} on:input={() => page = 0} /></label>
		<!-- svelte-ignore a11y-no-noninteractive-tabindex -->
		<div class="table-wrap" tabindex="0" role="region" aria-label={`${card.name} data`}>
			<table>
				<thead><tr>{#each columns as column}<th aria-sort={sortColumn === column ? descending ? 'descending' : 'ascending' : 'none'}><button on:click={() => sort(column)}>{label(column)}{sortColumn === column ? descending ? ' ↓' : ' ↑' : ''}</button></th>{/each}</tr></thead>
				<tbody>{#each sorted.slice(page * 25, (page + 1) * 25) as row}<tr>{#each columns as column}<td>{format(row[result.columns.indexOf(column)])}</td>{/each}</tr>{/each}</tbody>
			</table>
		</div>
		<div class="pagination"><span>{filtered.length} rows{result.truncated ? ' (first 1,000)' : ''}</span><button disabled={page === 0} on:click={() => page--}>Previous</button><span>{page + 1} / {pageCount}</span><button disabled={page + 1 >= pageCount} on:click={() => page++}>Next</button></div>
	{:else if card.display === 'scalar'}
		<div class="value">{format(result.rows[0][scalarIndex])}</div>
		<p class="scalar-label">{label(result.columns[scalarIndex])}</p>
	{:else}
		<div class="chart" class:pie={card.display === 'pie'} role="img" aria-label={card.name}><canvas use:metricChart={{ card, result }} aria-label={card.name}>Use the Data button to view chart values.</canvas></div>
		{#if result.truncated}<p class="error">Showing the first 1,000 rows. View Data for details.</p>{/if}
	{/if}
	{#if result}<footer>Updated {new Date(result.updatedAt).toLocaleTimeString()}</footer>{/if}
</article>

<style>
	article { grid-column: span var(--span); background: var(--surface); border: 1px solid var(--line); border-radius: var(--radius); padding: 14px; min-width: 0; }
	header { display: flex; align-items: flex-start; justify-content: space-between; gap: 8px; margin-bottom: 12px; }
	h3 { font-size: 1rem; margin: 0; text-align: left; }
	p { text-align: left; font-size: .85rem; color: var(--text-soft); }
	.actions { display: flex; gap: 5px; }
	button { background: var(--paper-warm); border: 1px solid var(--line); border-radius: 4px; color: var(--text); padding: 5px 8px; cursor: pointer; font: inherit; font-size: .75rem; white-space: nowrap; }
	button:hover:not(:disabled) { border-color: var(--forest); }
	button:disabled { opacity: .5; cursor: default; }
	.chart { position: relative; height: 380px; }
	.chart.pie { height: 460px; }
	.value { font-size: 2.5rem; font-weight: 800; color: var(--forest); padding: 14px 0 0; }
	.scalar-label { overflow-wrap: anywhere; font-size: .75rem; }
	.state { min-height: 80px; display: flex; align-items: center; }
	.error { color: var(--rust); }
	.search { display: flex; align-items: center; gap: 8px; font-size: .8rem; margin-bottom: 10px; }
	input { background: var(--surface); color: var(--text); border: 1px solid var(--line); border-radius: 4px; padding: 6px; min-width: 0; }
	.table-wrap { overflow: auto; max-height: 520px; }
	table { border-collapse: collapse; width: 100%; font-size: .8rem; text-align: left; }
	th { position: sticky; top: 0; background: var(--paper-warm); z-index: 1; }
	th button { border: 0; font-weight: 700; }
	td { border-bottom: 1px solid var(--line); padding: 8px; max-width: 420px; overflow-wrap: anywhere; white-space: pre-wrap; }
	.pagination { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; font-size: .75rem; margin-top: 10px; }
	.pagination span:first-child { margin-right: auto; }
	footer { color: var(--text-muted); font-size: .65rem; margin-top: 12px; }
	@media (max-width: 1050px) { article { grid-column: span 24; } article.scalar { grid-column: span 12; } .chart.pie { height: 400px; } }
	@media (max-width: 520px) { article.scalar { grid-column: span 24; } .chart { height: 340px; } }
</style>
