<script>

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

	async function resolveCrash(hash) {
		const res = await fetch(`https://api.clyde.games/resolvecrash?hash=` + hash, {
			method: 'GET'
		})
		
		promise = getCrashes();
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
					<div class="metadata">
						<p class="created">Last Seen: {new Date(crash.UpdatedAt).toLocaleString()}</p>
						<p class="created">Built At: {crash.Build}</p>
						<p class="created">Git Hash: <small>{crash.Commit}</small></p>
						<p class="created">DB Hash: <small>{crash.Hash}</small></p>
						<p class="created">Total: {crash.Count}</p>
						<p class="created">Env: {crash.Platform}</p>
						<button type="button" on:click={() => resolveCrash(crash.Hash)}>
							Resolve
						</button>

					</div>
					<div class="stack">
						<p class="message"><b>{crash.Message}</b></p>
						{#each getStack(crash) as stackMessage}
							<p class="message">{stackMessage}</p>
						{/each}
					</div>
				</div>
				<div class="comment-footer">
					<small>{crash.PID}:{crash.Project}:{crash.Env}</small>
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

	.metadata {
		display: flex;
		flex-direction: column;
		align-items: start;
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
		font-size: small;
	}

	.message {
		flex-grow: 1;
		text-align: start;
	}

	small {
		font-size: xx-small;
	}
</style>