<script>
    import Poetry from "./Poetry.svelte";

	async function getCrashes() {
		const res = await fetch(`https://api.clyde.games/crash`);
		const crashes = await res.json();

		if (res.ok) {
			return crashes;
		} else {
			throw new Error(crashes);
		}
	}

	let promise = getCrashes();

	function getStack(crash) {
		var stack = crash.Stack.split('FilePos');

		if(stack.length > 0 && stack[0] == '[') {
			stack.shift();
		}

		for(const i in stack) {
			stack[i] = stack[i].replaceAll('[', '');
			stack[i] = stack[i].replaceAll(']', '');
			stack[i] = stack[i].replaceAll('(', ' ');
			stack[i] = stack[i].replaceAll(')', ' ');
			stack[i] = stack[i].replaceAll(',', '::');
		}

		return stack;
	}

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
	<h2>Crash Reports</h2>
	<hr/>

	{#await promise}
		<p>loading...</p>
	{:then crashes}
		<div class="comment-list">
		{#each crashes as crash}
			<div class="comment">
				<div class="comment-body">
					<p class="created">{new Date(crash.CreatedAt).toLocaleString()}</p>
					<div class="stack">
						<p class="message"><b>{crash.Message}</b></p>
						{#each getStack(crash) as stackMessage}
							<p class="message">{stackMessage}</p>
						{/each}
					</div>
				</div>
				<div class="comment-footer">
					<small>{crash.PID}:{crash.Platform}:{crash.Project}:{crash.Env}</small>
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

	.stack {
		display: flex;
		flex-direction: column;
	}

	.comment-footer {
		display: flex;
		margin-top: 8px;
	}

	.comment p {
		margin: 4px;
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
</style>