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
			arbWord.Text = '';
			arbWord.Origin = 0;
		});

	}

	let promise = getUnknownWords();
	let arbWord = {
		Origin: 0,
		Text: ''
	};
</script>

<main>
	<h2>Assign Word Origins</h2>
	<hr/>

	{#await promise}
		<p>loading...</p>
	{:then words}
		<section>
			<div class="word-item">
				<input bind:value={arbWord.Text}/>
				<button on:click={() => updateWord(arbWord, 1)}>Germanic</button>
				<button on:click={() => updateWord(arbWord, 2)}>French</button>
				<button on:click={() => updateWord(arbWord, 3)}>Latin</button>
				<button on:click={() => updateWord(arbWord, 4)}>Greek</button>
				<button on:click={() => updateWord(arbWord, 5)}>Other</button>
			</div>
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
		<p style="color: var(--rust)">{error.message}</p>
	{/await}
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
		padding: 8px 0;
		border-bottom: 1px solid var(--line);
	}

	input {
		background: var(--surface);
		border: 1px solid var(--line);
		border-radius: var(--radius);
		color: var(--text);
		font: inherit;
		padding: 7px 10px;
	}

	button {
		background: var(--surface);
		border: 1px solid var(--line-strong);
		border-radius: var(--radius);
		color: var(--text);
		cursor: pointer;
		font: inherit;
		font-weight: 700;
		padding: 7px 10px;
	}

	button:hover {
		background: var(--paper-warm);
		border-color: var(--charcoal);
	}

	a {
		font-size:x-large;
	}
</style>
