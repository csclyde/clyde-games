<script>
	async function analyzeWords() {
		var body = {
			text: analyzedText,
		}

		const res = await fetch(`https://api.clyde.games/words/analyze`, {
			method: 'POST',
			body: JSON.stringify(body),
			headers: {
				'Content-Type': 'application/json'
			},
		});

		const words = await res.json();

		if (res.ok) {
			analyzedText = '';
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

	let analyzedText = '';

	let statsPromise = getWordStats();

	function pct(n) {
		return (Math.floor(n * 1000) / 10)
	}
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

	<textarea bind:value={analyzedText}></textarea>
	<button on:click={analyzeWords}>Analyze</button>
</main>

<style>
	main {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 16px;
	}

	textarea {
		width: 50%;
		height: 300px;
	}

	section {
		display: flex;
		gap: 16px;
	}
</style>