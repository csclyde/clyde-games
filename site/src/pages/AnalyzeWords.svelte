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

	let analyzedText = '';
</script>

<main>
	<h2>Analyze Word Origins</h2>
	<hr/>

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
</style>