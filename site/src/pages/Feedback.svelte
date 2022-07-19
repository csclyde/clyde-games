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
	<h2>Player Feedback</h2>
	<hr/>

	{#await promise}
		<p>loading...</p>
	{:then feedback}
		<div class="comment-list">
		{#each feedback as comment}
			<div class="comment">
				<div class="comment-body">
					<p class="rating" style="background-color:{ colors[comment.Rating] }"></p>
					<p class="message">{comment.Message}</p>
					<p class="created">{new Date(comment.CreatedAt).toLocaleString()}</p>
				</div>
				<div class="comment-footer">
					<small>{comment.PID}:{comment.Platform}:{comment.Project}:{comment.Env}</small>
					<small class="fps">FPS: {comment.FPS}</small>
				</div>
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
		flex-direction: column;
		border: 2px solid black;
		padding: 8px;
		border-radius: 6px;
	}

	.comment-body {
		display: flex;
		flex-direction: row;
		align-items: center;
	}

	.comment-footer {
		display: flex;
		margin-top: 8px;
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
		flex-shrink: 0;
	}

	.created {
		flex-shrink: 0;
	}

	.message {
		flex-grow: 1;
		text-align: start;
	}

	small {
		font-size: xx-small;
	}

	.fps {
		margin-left: auto;
	}
</style>