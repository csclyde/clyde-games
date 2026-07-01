<script>

	async function getCrashes() {
		const res = await fetch(`https://api.clyde.games/crash`);
		const crashes = await res.json();

		if (res.ok) {
			return crashes;
		} else {
			if (crashes.error == 'No crash found') {
				return [];
			}

			throw new Error(crashes.error || crashes);
		}
	}

	async function getAccessViolations() {
		const res = await fetch(`https://api.clyde.games/crash/accessviolations`);
		const crashes = await res.json();

		if (res.ok) {
			return crashes;
		} else {
			throw new Error(crashes.error || crashes);
		}
	}

	let crashPromise = getCrashes();
	let accessViolationPromise = getAccessViolations();

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

	function isAccessViolation(crash) {
		return crash.Message && crash.Message.toLowerCase().includes('access violation');
	}

	function getFirstStackLine(crash) {
		const stack = getStack(crash);
		const firstLine = stack.length > 0 && stack[0].trim() ? stack[0].trim() : 'Unknown location';
		return firstLine.split('::')[0].trim();
	}

	function getAccessViolationSummary(crashes) {
		const groups = {};

		for(const crash of crashes) {
			const firstLine = getFirstStackLine(crash);
			if(!groups[firstLine]) {
				groups[firstLine] = {
					firstLine,
					count: 0
				};
			}

			groups[firstLine].count += crash.Count || 1;
		}

		return Object.values(groups).sort((a, b) => b.count - a.count);
	}

	function getAccessViolationTotal(crashes) {
		return crashes
			.reduce((total, crash) => total + (crash.Count || 1), 0);
	}

	function getRegularCrashes(crashes) {
		return crashes.filter(crash => !isAccessViolation(crash));
	}

	async function resolveCrash(hash) {
		const res = await fetch(`https://api.clyde.games/resolvecrash?hash=` + hash, {
			method: 'GET'
		})
		
		crashPromise = getCrashes();
		accessViolationPromise = getAccessViolations();
	}

	async function resolveAllCrashes() {
		const res = await fetch(`https://api.clyde.games/resolvecrash?all=true`, {
			method: 'GET'
		})
		
		crashPromise = getCrashes();
		accessViolationPromise = getAccessViolations();
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
	<div class="controls">
		<button type="button" on:click={resolveAllCrashes}>
			Close All
		</button>
	</div>
	<hr/>

	<section class="access-violations">
		{#await accessViolationPromise}
			<div class="section-header">
				<h3>Access Violations</h3>
				<p>loading...</p>
			</div>
		{:then accessViolations}
			<div class="section-header">
				<h3>Access Violations</h3>
				<p>{getAccessViolationTotal(accessViolations)} last month</p>
			</div>
			{#if getAccessViolationTotal(accessViolations) > 0}
				<div class="access-list">
					{#each getAccessViolationSummary(accessViolations) as group}
						<div class="access-row">
							<strong>{group.count}</strong>
							<p>{group.firstLine}</p>
						</div>
					{/each}
				</div>
			{:else}
				<p class="empty-state">No Access Violations in the last month.</p>
			{/if}
		{:catch error}
			<div class="section-header">
				<h3>Access Violations</h3>
				<p>error</p>
			</div>
			<p class="empty-state">{error.message}</p>
		{/await}
	</section>

	{#await crashPromise}
		<p>loading...</p>
	{:then crashes}
		<div class="comment-list">
		{#each getRegularCrashes(crashes) as crash}
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

	.controls {
		display: flex;
		align-items: center;
		justify-content: flex-end;
		margin-bottom: 8px;
	}

	.access-violations {
		border: 2px solid black;
		border-radius: 6px;
		margin-bottom: 12px;
		padding: 8px;
	}

	.section-header {
		display: flex;
		align-items: center;
	}

	.section-header h3 {
		margin: 0;
	}

	.section-header p {
		margin: 0 0 0 auto;
		font-weight: 800;
	}

	.access-list {
		display: flex;
		flex-direction: column;
		gap: 4px;
		max-height: 180px;
		margin-top: 8px;
		overflow-y: auto;
	}

	.access-row {
		display: grid;
		grid-template-columns: 56px 1fr;
		align-items: center;
		gap: 8px;
		border-top: 1px solid #ddd;
		padding: 6px 0;
		text-align: start;
	}

	.access-row p {
		margin: 0;
	}

	.empty-state {
		margin: 8px 0 0;
		text-align: start;
	}
</style>
