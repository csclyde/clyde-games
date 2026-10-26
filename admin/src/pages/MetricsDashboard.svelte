<script>
	import { onMount, onDestroy } from 'svelte';
	import MetricCard from '../components/MetricCard.svelte';
	export let selectedProject = 'all';
	export let reportProjects = () => {};
	let cards = [];
	let states = {};
	let section = 'Levels';
	let loading = true;
	let error = '';
	let controller = new AbortController();
	$: visibleCards = cards.filter(card => card.section === section);
	$: sections = [...new Set(cards.map(card => card.section))].filter(name => name !== 'Other questions');
	async function read(url) {
		const response = await fetch(url, { signal: controller.signal });
		if (!response.ok) throw new Error(`Unable to load metrics (${response.status}). Please retry.`);
		return response.json();
	}
	async function loadCard(card) {
		if (states[card.id]?.loading || states[card.id]?.result) return;
		states = { ...states, [card.id]: { loading: true } };
		try {
			const result = await read(`/api/metrics/${card.id}?project=${encodeURIComponent(selectedProject)}`);
			states = { ...states, [card.id]: { result } };
		} catch (failure) {
			if (failure.name !== 'AbortError') states = { ...states, [card.id]: { error: failure.message } };
		}
	}
	async function loadSection() {
		const queue = cards.filter(card => card.section === section);
		async function worker() { while (queue.length && !controller.signal.aborted) await loadCard(queue.shift()); }
		await Promise.all([worker(), worker()]);
	}
	function selectSection(next) { section = next; loadSection(); }
	function refresh() {
		for (const card of visibleCards) {
			if (states[card.id]?.loading) return;
		}
		states = Object.fromEntries(Object.entries(states).filter(([id]) => !visibleCards.some(card => card.id === Number(id))));
		loadSection();
	}
	async function initialize() {
		loading = true; error = '';
		try { cards = await read('/api/metrics'); await loadSection(); }
		catch (failure) { if (failure.name !== 'AbortError') error = failure.message; }
		finally { loading = false; }
	}
	onMount(() => {
		initialize();
		read('https://api.clyde.games/projects').then(projects => reportProjects('charts', projects.map(p => p.id))).catch(() => {});
	});
	onDestroy(() => controller.abort());
</script>

<main>
	<header>
		<div><h2>Metrics</h2><p>Gameplay analytics · {selectedProject === 'all' ? 'All projects' : selectedProject}</p></div>
		<div><p class="cache-note">Results cached for up to 5 minutes.</p><button on:click={refresh} disabled={loading || visibleCards.some(card => states[card.id]?.loading)}>Refresh</button></div>
	</header>
	{#if error}<p class="error" role="alert">{error} <button on:click={initialize}>Retry</button></p>{/if}
	{#if loading && !cards.length}<p role="status">Loading metrics…</p>{/if}
	<nav aria-label="Metric groups">{#each sections as name}<button class:active={section === name} aria-pressed={section === name} on:click={() => selectSection(name)}>{name}</button>{/each}</nav>
	<div class="grid">{#each visibleCards as card (card.id)}<MetricCard {card} state={states[card.id] || {}} retry={() => loadCard(card)} />{/each}</div>
</main>

<style>
	main { max-width: 1368px; margin: 12px auto 32px; padding: 0 16px; color: var(--text); }
	header { display: flex; justify-content: space-between; align-items: center; gap: 12px; margin: 20px 0; }
	h2 { font-size: 1.5rem; margin: 0; text-align: left; }
	p { color: var(--text-muted); font-size: .82rem; margin: 5px 0; text-align: left; }
	.cache-note { font-size: .75rem; }
	nav { display: flex; gap: 6px; margin-bottom: 14px; flex-wrap: wrap; }
	button { background: var(--surface); border: 1px solid var(--line); border-radius: 4px; padding: 8px 12px; color: var(--text); font: inherit; font-size: .8rem; cursor: pointer; }
	button.active { color: var(--surface); background: var(--forest); border-color: var(--forest); }
	.grid { display: grid; grid-template-columns: repeat(24, minmax(0, 1fr)); gap: 12px; align-items: start; }
	.error { color: var(--rust); }
	@media (max-width: 520px) { main { padding: 0 8px; } header { align-items: flex-start; flex-direction: column; } }
</style>
