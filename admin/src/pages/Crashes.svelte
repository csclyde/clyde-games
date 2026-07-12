<script>
	import PlankaTicketModal from '../components/PlankaTicketModal.svelte';

	export let selectedProject = 'cursemark';
	export let reportProjects = () => {};

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
	let helperStatus = {};
	let ticketCrash = null;

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

		for(const crash of getRecentAccessViolations(crashes)) {
			const firstLine = getFirstStackLine(crash);
			if(!groups[firstLine]) {
				groups[firstLine] = {
					firstLine,
					count: 0
				};
			}

			groups[firstLine].count += 1;
		}

		return Object.values(groups).sort((a, b) => b.count - a.count);
	}

	function getAccessViolationTotal(crashes) {
		return getRecentAccessViolations(crashes).length;
	}

	function getAccessViolationPlayerTotal(crashes) {
		const players = new Set();

		for(const crash of getRecentAccessViolations(crashes)) {
			if(crash.PID) {
				players.add(crash.PID);
			}
		}

		return players.size;
	}

	function getRecentAccessViolations(crashes) {
		const oneMonthAgo = new Date();
		oneMonthAgo.setMonth(oneMonthAgo.getMonth() - 1);

		return crashes.filter(crash => {
			const lastSeen = new Date(crash.UpdatedAt);

			return !isNaN(lastSeen.getTime()) && lastSeen >= oneMonthAgo;
		});
	}

	function getRegularCrashes(crashes) {
		return crashes.filter(crash => !isAccessViolation(crash));
	}

	function getCrashKey(crash) {
		return crash.ID || crash.Hash || `${crash.Message || ''}:${crash.Stack || ''}`;
	}

	function getVisibleStack(crash) {
		const stack = getStack(crash);

		if (crash.expanded) {
			return stack;
		}

		return stack.slice(0, 6);
	}

	function shouldShowExpand(crash) {
		return !crash.expanded && getStack(crash).length > 6;
	}

	function expandCrash(crash) {
		const crashKey = getCrashKey(crash);
		crashes = crashes.map(item => getCrashKey(item) === crashKey
			? { ...item, expanded: true }
			: item);
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

	function matchesProject(item) {
		return selectedProject === 'all' || (item.Project || '').toLowerCase() === selectedProject.toLowerCase();
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

	function openTicketModal(crash) {
		ticketCrash = crash;
	}

	function ticketName(crash) {
		const source = crash.Message || getFirstStackLine(crash) || 'Crash report';
		const firstLine = source.split('\n')[0].trim();
		return firstLine.length > 120 ? firstLine.slice(0, 117) + '...' : firstLine;
	}

	function ticketDescription(crash) {
		const details = [
			crash.Message || '',
			'',
			'---',
			`Crash Hash: ${metadataValue(crash.Hash)}`,
			`Project: ${metadataValue(crash.Project)}`,
			`Environment: ${metadataValue(crash.Env)}`,
			`Platform: ${metadataValue(crash.Platform)}`,
			`Category: ${metadataValue(crash.Category)}`,
			`Build: ${metadataValue(crash.Build)}`,
			`Commit: ${metadataValue(crash.Commit)}`,
			`Player ID: ${metadataValue(crash.PID)}`,
			`Count: ${metadataValue(crash.Count)}`,
			`Last Seen: ${formatDate(crash.UpdatedAt)}`,
			'',
			'Stack:',
			...getStack(crash)
		];

		return details.join('\n');
	}

	async function handleTicketCreated() {
		const crash = ticketCrash;
		if (!crash) {
			return;
		}

		helperStatus = { ...helperStatus, [crash.Hash]: { status: 'Planka ticket created', error: '' } };
		ticketCrash = null;
		await resolveCrash(crash.Hash);
	}

	async function resolveAllCrashes() {
		resolvingAllCrashes = true;

		try {
			const crashesToResolve = visibleRegularCrashes;
			await Promise.all(crashesToResolve.map(crash => fetch(`https://api.clyde.games/resolvecrash?hash=` + crash.Hash, {
				method: 'GET'
			})));

			crashes = crashes.filter(crash => !crashesToResolve.some(item => item.Hash === crash.Hash));
			await refreshCrashLists();
		} finally {
			resolvingAllCrashes = false;
		}
	}

	$: visibleCrashes = crashes.filter(matchesProject);
	$: visibleAccessViolations = accessViolations.filter(matchesProject);
	$: visibleRegularCrashes = getRegularCrashes(visibleCrashes);
	$: reportProjects('crashes', [
		...crashes.map(crash => crash.Project),
		...accessViolations.map(crash => crash.Project)
	]);
</script>

<main>
	<header class="page-header">
		<div>
			<h2>Crash Reports</h2>
			<p>{visibleRegularCrashes.length} open item{visibleRegularCrashes.length === 1 ? '' : 's'}</p>
		</div>
		<button type="button" disabled={resolvingAllCrashes || visibleRegularCrashes.length === 0} on:click={resolveAllCrashes}>
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
				<p>{getAccessViolationTotal(visibleAccessViolations)} incident{getAccessViolationTotal(visibleAccessViolations) === 1 ? '' : 's'} / {getAccessViolationPlayerTotal(visibleAccessViolations)} player{getAccessViolationPlayerTotal(visibleAccessViolations) === 1 ? '' : 's'} last month</p>
			</div>
			{#if getAccessViolationTotal(visibleAccessViolations) > 0}
				<div class="access-list">
					{#each getAccessViolationSummary(visibleAccessViolations) as group}
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
	{:else if visibleRegularCrashes.length === 0}
		<p class="state-message">No open crashes.</p>
	{:else}
		<div class="comment-list">
		{#each visibleRegularCrashes as crash (getCrashKey(crash))}
			<div class:resolving={resolvingCrashes[crash.Hash] || resolvingAllCrashes} class="comment">
				<div class="comment-body">
					<div class="comment-main">
						<div class="stack">
							<p class="message"><b>{crash.Message}</b></p>
							{#each getVisibleStack(crash) as stackMessage}
								<p class="message">{stackMessage}</p>
							{/each}
							{#if helperStatus[crash.Hash] && helperStatus[crash.Hash].status}
								<small class="helper-status">{helperStatus[crash.Hash].status}</small>
							{/if}
							{#if helperStatus[crash.Hash] && helperStatus[crash.Hash].error}
								<small class="helper-error">{helperStatus[crash.Hash].error}</small>
							{/if}
							{#if shouldShowExpand(crash)}
								<button class="expand-button" type="button" on:click={() => expandCrash(crash)}>
									Expand...
								</button>
							{/if}
						</div>
						<div class="meta-list">
							{#if crash.Project}<span>project: {crash.Project}</span>{/if}
							{#if crash.Env}<span>env: {crash.Env}</span>{/if}
							{#if crash.Platform}<span>platform: {crash.Platform}</span>{/if}
							{#if crash.Category}<span>category: {crash.Category}</span>{/if}
							{#if crash.PID}
								<a class="user-id" href={`/user/${encodeURIComponent(crash.PID)}`}>user: {crash.PID}</a>
							{/if}
							{#if crash.Commit}
								<span>commit: {crash.Commit}</span>
							{/if}
							{#if crash.Hash}
								<span>hash: {crash.Hash}</span>
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
							<button type="button" disabled={resolvingCrashes[crash.Hash] || resolvingAllCrashes} on:click={() => openTicketModal(crash)}>
								Make Ticket
							</button>
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

<PlankaTicketModal
	open={!!ticketCrash}
	title={ticketCrash ? ticketName(ticketCrash) : ''}
	description={ticketCrash ? ticketDescription(ticketCrash) : ''}
	on:close={() => (ticketCrash = null)}
	on:created={handleTicketCreated}
/>

<style>
	main {
		color: var(--text);
		max-width: 1400px;
		margin: 0 auto;
		padding: 12px 16px 32px;
	}

	.page-header {
		align-items: flex-end;
		border-bottom: 1px solid var(--line);
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
		color: var(--olive);
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
		background: var(--surface);
		border: 1px solid var(--line);
		border-left: 4px solid var(--forest);
		border-radius: var(--radius);
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
		color: var(--text-muted);
		display: grid;
		font-size: 0.82rem;
		gap: 2px;
		line-height: 1.25;
		text-align: left;
	}

	.stats span {
		color: var(--text-soft);
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
		background: var(--paper-warm);
		border: 1px solid var(--line);
		border-radius: 4px;
		color: var(--text-muted);
		font-size: 0.68rem;
		line-height: 1.25;
		max-width: 100%;
		overflow-wrap: anywhere;
		padding: 3px 6px;
		text-align: left;
	}

	.user-id {
		background: var(--paper-warm);
		border: 1px solid var(--line);
		border-radius: 4px;
		color: var(--text-muted);
		font-size: 0.68rem;
		line-height: 1.25;
		max-width: 100%;
		overflow-wrap: anywhere;
		padding: 3px 6px;
		text-align: left;
		text-decoration: none;
	}

	.user-id:hover {
		background: var(--surface);
		border-color: var(--charcoal);
		color: var(--text);
	}

	.message {
		color: var(--charcoal);
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

	.helper-status {
		color: var(--text-soft);
		font-weight: 800;
		letter-spacing: 0.06em;
		text-transform: uppercase;
	}

	.helper-error {
		color: var(--rust);
		max-width: 520px;
	}

	button {
		background: var(--surface);
		border: 1px solid var(--line-strong);
		border-radius: 4px;
		box-sizing: border-box;
		color: var(--text);
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
		background: var(--paper-warm);
		border-color: var(--charcoal);
	}

	button:disabled {
		color: var(--text-soft);
		cursor: default;
	}

	.expand-button {
		align-self: flex-start;
		background: transparent;
		border: 0;
		color: var(--text-muted);
		font-size: 0.85rem;
		font-weight: 700;
		line-height: 1.35;
		margin-top: 8px;
		padding: 0;
		text-decoration: underline;
		text-underline-offset: 2px;
	}

	.expand-button:hover:not(:disabled) {
		background: transparent;
		border-color: transparent;
		color: var(--rust);
	}

	.access-violations {
		background: var(--surface);
		border: 1px solid var(--line);
		border-left: 4px solid var(--safety);
		border-radius: var(--radius);
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
		color: var(--olive);
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
		border-top: 1px solid var(--line);
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

	small {
		font-size: 0.68rem;
		line-height: 1.3;
		overflow-wrap: anywhere;
		text-align: left;
	}

	.state-message {
		color: var(--text-soft);
		margin: 32px 0;
		text-align: left;
	}

	.error {
		color: var(--rust);
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
			border: 1px solid var(--line-strong);
			border-top: 4px solid var(--forest);
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
