<script>
	async function getUnknownWords() {
		const res = await fetch(`https://api.clyde.games/words/unknown`, {
			method: 'POST',
		});

		const words = await res.json();

		if (res.ok) {
			return words;
		} else {
			throw new Error(words);
		}
	}

	async function updateWord(w, o) {
		w.Origin = o;
		var body = [w];

		const res = await fetch(`https://api.clyde.games/words/update`, {
			method: 'POST',
			body: JSON.stringify(body),
			headers: {
				'Content-Type': 'application/json'
			},
		}).then(r => {
			promise = getUnknownWords();
		});
	}

	let promise = getUnknownWords();
</script>

<main>
	<h2>Assign Word Origins</h2>
	<hr/>

	{#await promise}
		<p>loading...</p>
	{:then words}
		<section>
		{#each words as word}
			<div class="word-item">
				<a target="blank" href='https://etymonline.com/word/{word.Text}'>{word.Text}</a>
				<button on:click={() => updateWord(word, 1)}>Germanic</button>
				<button on:click={() => updateWord(word, 2)}>French</button>
				<button on:click={() => updateWord(word, 3)}>Latin</button>
				<button on:click={() => updateWord(word, 4)}>Greek</button>
				<button on:click={() => updateWord(word, 5)}>Other</button>
				<button on:click={() => updateWord(word, -1)}>Nonsense</button>
			</div>
		{/each}
		</section>
	{:catch error}
		<p style="color: red">{error.message}</p>
	{/await}
</main>

<style>
	main {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 16px;
	}

	section {
		display: flex;
		flex-direction: column;
		align-items: flex-end;
		gap: 16px;
	}

	.word-item {
		display: flex;
		flex-direction: row;
		gap: 8px;
		padding: 4px;
		border-bottom: 1px solid;
	}

	a {
		font-size:x-large;
	}
</style>