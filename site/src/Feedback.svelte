<script>

	async function getFeedback() {
		const res = await fetch(`https://api.clyde.games/feedback`);
		const feedback = await res.json();

		if (res.ok) {
			return feedback;
		} else {
			throw new Error(feedback);
		}
	}

	let promise = getFeedback();

	let colors = [
		'black',
		'red',
		'orange',
		'gray',
		'blue',
		'green'
	]

</script>

<main>
	<h1>Player Feedback</h1>
	<hr/>

	{#await promise}
		<p>loading...</p>
	{:then feedback}
		<div class="comment-list">
		{#each feedback as comment}
			<div class="comment">
				<p class="rating" style="background-color:{ colors[comment.Rating] }"></p>
				<p class="message">{comment.Message}</p>
				<p class="created">{new Date(comment.CreatedAt).toLocaleString()}</p>
			</div>
		{/each}
		</div>
	{:catch error}
		<p style="color: red">{error.message}</p>
	{/await}
</main>

<style>
	.comment-list {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.comment {
		display: flex;
		flex-direction: row;
		border: 2px solid black;
		border-radius: 6px;
		padding: 8px;
	}

	.comment p {
		margin: 4px;
	}

	.rating {
		/* background-color: red; */
		border-radius: 50%;
		width: 20px;
		height: 20px;
		text-align: center;
		font-weight: 800;
	}

	.message {
		flex-grow: 1;
	}
</style>