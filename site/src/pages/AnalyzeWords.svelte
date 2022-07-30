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
		'black',
		'green',
		'blue',
		'orange',
		'red',
		'gray'
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
		<p style="color: red">{error.message}</p>
	{/await}

	<textarea bind:value={analysisText}></textarea>
	<button on:click={() => analyzePromise = analyzeWords()}>Analyze</button>

	{#if analyzePromise != null}
	{#await analyzePromise}
	<p>loading...</p>
	{:then analysis}
	<section>
		<p style="color:{ colors[0] }">Unknown: {pct(analysis.Statistics.Unknown / analysisText.Statistics.Total)}</p>
		<p style="color:{ colors[1] }">Germanic: {pct(analysis.Statistics.Germanic / analysisText.Statistics.Total)}</p>
		<p style="color:{ colors[2] }">French: {pct(analysis.Statistics.French / analysisText.Statistics.Total)}</p>
		<p style="color:{ colors[3] }">Latin: {pct(analysis.Statistics.Latin / analysisText.Statistics.Total)}</p>
		<p style="color:{ colors[4] }">Greek: {pct(analysis.Statistics.Greek / analysisText.Statistics.Total)}</p>
		<p style="color:{ colors[5] }">Other: {pct(analysis.Statistics.Other / analysisText.Statistics.Total)}</p>
	</section>
	<div class="results">
		{#each analysis.Words as word}
		<small style="color:{ colors[word.Origin] }">{word.Text}</small>
		{/each}
	</div>
	{:catch error}
	<p style="color: red">{error.message}</p>
	{/await}
	{/if}

</main>

<style>
	main {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 16px;
	}

	textarea {
		width: 75%;
		height: 300px;
	}

	section {
		display: flex;
		gap: 16px;
		flex-wrap: wrap;
	}

	.results {
		display: flex;
		flex-wrap: wrap;
		max-width: 75%;
		gap: 4px;
	}
</style>