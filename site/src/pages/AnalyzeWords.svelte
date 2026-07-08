<script>
	async function analyzeWords() {
		var body = {
			text: analysisText,
		}

		analysisText = '';

		const res = await fetch(`https://api.clyde.games/words/analyze`, {
			method: 'POST',
			body: JSON.stringify(body),
			headers: {
				'Content-Type': 'application/json'
			},
		});

		const words = await res.json();

		if (res.ok) {
			return words;
		} else {
			throw new Error(words);
		}
	}

	async function getWordStats() {
		const res = await fetch(`https://api.clyde.games/words/stats`, {
			method: 'POST',
		});

		const stats = await res.json();

		if (res.ok) {
			return stats;
		} else {
			throw new Error(stats);
		}
	}

	let analysisText = '';
	let analyzePromise = null;

	let statsPromise = getWordStats();

	function pct(n) {
		return (Math.floor(n * 1000) / 10)
	}

	let colors = [
		'var(--charcoal)',
		'var(--forest)',
		'var(--slate)',
		'var(--ochre)',
		'var(--rust)',
		'var(--text-soft)'
	]
</script>

<main>
	<h2>Analyze Word Origins</h2>
	<hr/>
	{#await statsPromise}
		<p>loading...</p>
	{:then stats}
	<section>
		<small>Total: {stats.Total}</small>
		<small>Unknown: {pct(stats.Unknown / stats.Total)}%</small>
		<small>Germanic: {pct(stats.Germanic / stats.Total)}%</small>
		<small>French: {pct(stats.French / stats.Total)}%</small>
		<small>Latin: {pct(stats.Latin / stats.Total)}%</small>
		<small>Greek: {pct(stats.Greek / stats.Total)}%</small>
		<small>Other: {pct(stats.Other / stats.Total)}%</small>
	</section>
	{:catch error}
		<p style="color: var(--rust)">{error.message}</p>
	{/await}

	<textarea bind:value={analysisText}></textarea>
	<button on:click={() => analyzePromise = analyzeWords()}>Analyze</button>

	{#if analyzePromise != null}
	{#await analyzePromise}
	<p>loading...</p>
	{:then analysis}
	<section>
		<p style="color:{ colors[0] }">Unknown: {pct(analysis.Statistics.Unknown / analysis.Statistics.Total)}%</p>
		<p style="color:{ colors[1] }">Germanic: {pct(analysis.Statistics.Germanic / analysis.Statistics.Total)}%</p>
		<p style="color:{ colors[2] }">French: {pct(analysis.Statistics.French / analysis.Statistics.Total)}%</p>
		<p style="color:{ colors[3] }">Latin: {pct(analysis.Statistics.Latin / analysis.Statistics.Total)}%</p>
		<p style="color:{ colors[4] }">Greek: {pct(analysis.Statistics.Greek / analysis.Statistics.Total)}%</p>
		<p style="color:{ colors[5] }">Other: {pct(analysis.Statistics.Other / analysis.Statistics.Total)}%</p>
	</section>
	<div class="results">
		{#each analysis.Words as word}
		<a target="_blank" href='https://www.etymonline.com/word/{word.Text}' style="color:{ colors[word.Origin] }">{word.Text}</a>
		{/each}
	</div>
	{:catch error}
	<p style="color: var(--rust)">{error.message}</p>
	{/await}
	{/if}

</main>

<style>
	main {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 16px;
		margin: 0 auto;
		max-width: var(--content);
		padding: 64px 24px 24px;
	}

	h2 {
		text-align: left;
		width: 100%;
	}

	hr {
		border: 0;
		border-top: 1px solid var(--line);
		width: 100%;
	}

	textarea {
		background: var(--surface);
		border: 1px solid var(--line);
		border-radius: var(--radius);
		color: var(--text);
		font: inherit;
		width: 75%;
		height: 300px;
		padding: 12px;
	}

	section {
		display: flex;
		gap: 16px;
		flex-wrap: wrap;
	}

	button {
		background: var(--surface);
		border: 1px solid var(--line-strong);
		border-radius: var(--radius);
		color: var(--text);
		cursor: pointer;
		font: inherit;
		font-weight: 700;
		padding: 8px 12px;
	}

	button:hover {
		background: var(--paper-warm);
		border-color: var(--charcoal);
	}

	.results {
		display: flex;
		flex-wrap: wrap;
		max-width: 75%;
		gap: 4px;
	}
</style>
