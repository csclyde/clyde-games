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

	let crashes = [];
	let accessViolations = [];
	let loadingCrashes = true;
	let loadingAccessViolations = true;
	let crashError = '';
	let accessViolationError = '';
	let resolvingCrashes = {};
	let resolvingAllCrashes = false;
	let expandedCrashes = {};

	async function loadCrashes() {
		loadingCrashes = true;
		crashError = '';

		try {
			crashes = await getCrashes();
		} catch (error) {
			crashError = error.message;
		} finally {
			loadingCrashes = false;
		}
	}

	async function loadAccessViolations() {
		loadingAccessViolations = true;
		accessViolationError = '';

		try {
			accessViolations = await getAccessViolations();
		} catch (error) {
			accessViolationError = error.message;
		} finally {
			loadingAccessViolations = false;
		}
	}

	loadCrashes();
	loadAccessViolations();

	async function refreshCrashLists() {
		try {
			crashes = await getCrashes();
			crashError = '';
		} catch (error) {
			crashError = error.message;
		}

		try {
			accessViolations = await getAccessViolations();
			accessViolationError = '';
		} catch (error) {
			accessViolationError = error.message;
		}
	}

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
		return stack.length > 0 && stack[0].trim() ? stack[0].trim() : 'Unknown location';
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

	function getVisibleStack(crash) {
		const stack = getStack(crash);

		if (expandedCrashes[crash.Hash]) {
			return stack;
		}

		return stack.slice(0, 6);
	}

	function shouldShowExpand(crash) {
		return !expandedCrashes[crash.Hash] && getStack(crash).length > 6;
	}

	function expandCrash(hash) {
		expandedCrashes = { ...expandedCrashes, [hash]: true };
	}

	function formatDate(value) {
		const date = new Date(value);

		if (isNaN(date.getTime())) {
			return value || '-';
		}

		return date.toLocaleString(undefined, {
			year: 'numeric',
			month: 'short',
			day: 'numeric',
			hour: 'numeric',
			minute: '2-digit'
		});
	}

	function metadataValue(value) {
		return value || 'unknown';
	}

	function formatBuildDate(value) {
		if (!value) {
			return metadataValue(value);
		}

		return formatDate(value);
	}

	function crashContext(crash) {
		return [crash.Project, crash.Env, crash.Platform, crash.Category].filter(Boolean).join(' / ');
	}

	async function resolveCrash(hash) {
		resolvingCrashes = { ...resolvingCrashes, [hash]: true };

		try {
			const res = await fetch(`https://api.clyde.games/resolvecrash?hash=` + hash, {
				method: 'GET'
			});

			if (res.ok) {
				crashes = crashes.filter(crash => crash.Hash !== hash);
				await refreshCrashLists();
			}
		} finally {
			const nextResolvingCrashes = { ...resolvingCrashes };
			delete nextResolvingCrashes[hash];
			resolvingCrashes = nextResolvingCrashes;
		}
	}

	async function resolveAllCrashes() {
		resolvingAllCrashes = true;

		try {
			const res = await fetch(`https://api.clyde.games/resolvecrash?all=true`, {
				method: 'GET'
			});

			if (res.ok) {
				crashes = [];
				await refreshCrashLists();
			}
		} finally {
			resolvingAllCrashes = false;
		}
	}

</script>

<main>
	<header class="page-header">
		<div>
			<h2>Crash Reports</h2>
			<p>{getRegularCrashes(crashes).length} open item{getRegularCrashes(crashes).length === 1 ? '' : 's'}</p>
		</div>
		<button type="button" disabled={resolvingAllCrashes || getRegularCrashes(crashes).length === 0} on:click={resolveAllCrashes}>
			{resolvingAllCrashes ? 'Closing...' : 'Close All'}
		</button>
	</header>

	<section class="access-violations">
		{#if loadingAccessViolations}
			<div class="section-header">
				<h3>Access Violations</h3>
				<p>loading...</p>
			</div>
		{:else if accessViolationError}
			<div class="section-header">
				<h3>Access Violations</h3>
				<p>error</p>
			</div>
			<p class="empty-state">{accessViolationError}</p>
		{:else}
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
		{/if}
	</section>

	{#if loadingCrashes}
		<p class="state-message">loading...</p>
	{:else if crashError}
		<p class="state-message error">{crashError}</p>
	{:else if getRegularCrashes(crashes).length === 0}
		<p class="state-message">No open crashes.</p>
	{:else}
		<div class="comment-list">
		{#each getRegularCrashes(crashes) as crash}
			<div class:resolving={resolvingCrashes[crash.Hash] || resolvingAllCrashes} class="comment">
				<div class="comment-body">
					<div class="comment-main">
						<div class="stack">
							<p class="message"><b>{crash.Message}</b></p>
							{#each getVisibleStack(crash) as stackMessage}
								<p class="message">{stackMessage}</p>
							{/each}
							{#if shouldShowExpand(crash)}
								<button class="expand-button" type="button" on:click={() => expandCrash(crash.Hash)}>
									Expand...
								</button>
							{/if}
						</div>
						<div class="meta-list">
							{#if crashContext(crash)}
								<span>{crashContext(crash)}</span>
							{/if}
							{#if crash.PID}
								<span>{crash.PID}</span>
							{/if}
							{#if crash.Commit}
								<span>{crash.Commit}</span>
							{/if}
							{#if crash.Hash}
								<span>{crash.Hash}</span>
							{/if}
						</div>
					</div>
					<div class="comment-side">
						<div class="stats">
							<p><span>Last Seen</span>{formatDate(crash.UpdatedAt)}</p>
							<p><span>Build</span>{formatBuildDate(crash.Build)}</p>
							<p><span>Total</span>{crash.Count}</p>
						</div>
						<div class="actions">
							<button type="button" disabled={resolvingCrashes[crash.Hash] || resolvingAllCrashes} on:click={() => resolveCrash(crash.Hash)}>
								{resolvingCrashes[crash.Hash] ? 'Resolving...' : 'Resolve'}
							</button>
						</div>
					</div>
				</div>
			</div>
		{/each}
		</div>
	{/if}
</main>

<style>
	main {
		color: #171717;
		max-width: 1400px;
		margin: 0 auto;
		padding: 12px 16px 32px;
	}

	.page-header {
		align-items: flex-end;
		border-bottom: 1px solid #d8d8d8;
		display: flex;
		gap: 16px;
		justify-content: space-between;
		margin-bottom: 14px;
		padding-bottom: 12px;
	}

	.page-header h2 {
		font-size: 2.5rem;
		line-height: 1;
		text-align: left;
	}

	.page-header p {
		color: #666;
		font-size: 0.85rem;
		font-weight: 600;
		letter-spacing: 0.02em;
		margin: 6px 0 0;
		text-align: left;
		text-transform: uppercase;
	}

	.comment-list {
		display: flex;
		flex-direction: column;
		gap: 10px;
	}

	.comment {
		background: #fff;
		border: 1px solid #bdbdbd;
		border-left: 4px solid #111;
		border-radius: 4px;
		box-sizing: border-box;
		padding: 14px 16px;
		transition: opacity 120ms ease, background-color 120ms ease;
	}

	.comment.resolving {
		opacity: 0.45;
		pointer-events: none;
	}

	.comment-body {
		align-items: stretch;
		display: grid;
		gap: 18px;
		grid-template-columns: minmax(0, 1fr) minmax(320px, auto);
	}

	.comment-main {
		display: flex;
		flex-direction: column;
		gap: 10px;
		min-width: 0;
	}

	.comment-side {
		align-items: flex-end;
		display: flex;
		flex-direction: column;
		gap: 14px;
	}

	.stack {
		display: flex;
		flex-direction: column;
		min-width: 0;
	}

	.comment p {
		margin: 0;
	}

	.stats {
		display: grid;
		gap: 12px;
		grid-template-columns: repeat(3, minmax(90px, max-content));
		justify-content: end;
	}

	.stats p {
		color: #333;
		display: grid;
		font-size: 0.82rem;
		gap: 2px;
		line-height: 1.25;
		text-align: left;
	}

	.stats span {
		color: #777;
		font-size: 0.68rem;
		font-weight: 800;
		letter-spacing: 0.06em;
		text-transform: uppercase;
	}

	.meta-list {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
	}

	.meta-list span {
		background: #f4f4f4;
		border: 1px solid #ddd;
		border-radius: 4px;
		color: #555;
		font-size: 0.68rem;
		line-height: 1.25;
		max-width: 100%;
		overflow-wrap: anywhere;
		padding: 3px 6px;
		text-align: left;
	}

	.message {
		color: #111;
		flex-grow: 1;
		font-size: 0.9rem;
		line-height: 1.35;
		overflow-wrap: anywhere;
		text-align: left;
	}

	.message:first-child {
		font-size: 1rem;
		margin-bottom: 8px;
	}

	button {
		background: #fff;
		border: 1px solid #a8a8a8;
		border-radius: 4px;
		box-sizing: border-box;
		color: #111;
		cursor: pointer;
		font: inherit;
		font-size: 0.82rem;
		font-weight: 700;
		line-height: 1;
		padding: 7px 10px;
	}

	.actions {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
		justify-content: flex-end;
	}

	button:hover:not(:disabled) {
		background: #f2f2f2;
		border-color: #666;
	}

	button:disabled {
		color: #777;
		cursor: default;
	}

	.expand-button {
		align-self: flex-start;
		margin-top: 8px;
	}

	.access-violations {
		border: 1px solid #bdbdbd;
		border-left: 4px solid #111;
		border-radius: 4px;
		box-sizing: border-box;
		margin-bottom: 12px;
		padding: 14px 16px;
	}

	.section-header {
		display: flex;
		align-items: center;
	}

	.section-header h3 {
		font-size: 1rem;
		margin: 0;
		text-align: left;
	}

	.section-header p {
		color: #666;
		font-size: 0.85rem;
		letter-spacing: 0.02em;
		margin: 0 0 0 auto;
		font-weight: 800;
		text-align: right;
		text-transform: uppercase;
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
		padding: 7px 0;
		text-align: left;
	}

	.access-row p {
		margin: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.empty-state {
		margin: 8px 0 0;
		text-align: left;
	}

	.state-message {
		color: #666;
		margin: 32px 0;
		text-align: left;
	}

	.error {
		color: #a40000;
	}

	@media (max-width: 820px) {
		main {
			padding: 8px 4px 24px;
		}

		.page-header {
			align-items: flex-start;
			flex-direction: column;
		}

		.page-header h2 {
			font-size: 2rem;
		}

		.comment-list {
			gap: 12px;
		}

		.comment,
		.access-violations {
			border: 1px solid #a8a8a8;
			border-top: 4px solid #111;
			border-left-width: 1px;
			border-radius: 6px;
			padding: 12px;
			width: 100%;
		}

		.comment-body {
			grid-template-columns: 1fr;
		}

		.comment-side {
			align-items: flex-start;
			width: 100%;
		}

		.stats {
			grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
			justify-content: stretch;
			width: 100%;
		}

		.actions {
			justify-content: flex-start;
			width: 100%;
		}

		.actions button {
			flex: 1 1 96px;
		}

		.stack {
			width: 100%;
		}

		.access-row {
			grid-template-columns: 44px 1fr;
		}

		.access-row p {
			overflow-wrap: anywhere;
			white-space: normal;
		}
	}

	@media (max-width: 480px) {
		.stats {
			grid-template-columns: 1fr;
		}

		.section-header {
			align-items: flex-start;
			flex-direction: column;
			gap: 4px;
		}

		.section-header p {
			margin-left: 0;
			text-align: left;
		}
	}
</style>
