<script>
	export let selectedProject = 'all';
	export let reportProjects = () => {};

	async function parseResponse(res) {
		const text = await res.text();

		try {
			return JSON.parse(text);
		} catch (error) {
			if (res.ok) {
				throw new Error('Expected JSON response from metrics API');
			}

			throw new Error(text || res.statusText);
		}
	}

	async function getBuilds() {
		const res = await fetch(`https://api.clyde.games/event/builds`);
		const builds = await parseResponse(res);

		if (res.ok) {
			return builds;
		} else {
			throw new Error(builds.error || builds);
		}
	}

	async function getSavegameBuilds() {
		const res = await fetch(`https://api.clyde.games/savegame/builds`);
		const builds = await parseResponse(res);

		if (res.ok) {
			return builds;
		} else {
			throw new Error(builds.error || builds);
		}
	}

	async function getEventSettings() {
		const res = await fetch(`https://api.clyde.games/event/settings`);
		const settings = await parseResponse(res);

		if (res.ok) {
			return settings;
		} else {
			throw new Error(settings.error || settings);
		}
	}

	let builds = [];
	let savegameBuilds = [];
	let loadingBuilds = true;
	let loadingSavegameBuilds = true;
	let buildError = '';
	let savegameBuildError = '';
	let deletingEventBuild = null;
	let deletingSavegameBuild = null;
	let deleteError = '';
	let oldestBuild = '';
	let settingsError = '';
	let savingOldestBuild = false;
	let oldestBuildSaved = false;

	async function loadBuilds() {
		loadingBuilds = true;
		loadingSavegameBuilds = true;
		buildError = '';
		savegameBuildError = '';

		const [eventBuilds, savegameBuildItems, eventSettings] = await Promise.allSettled([
			getBuilds(),
			getSavegameBuilds(),
			getEventSettings()
		]);

		if (eventBuilds.status === 'fulfilled') {
			builds = eventBuilds.value;
		} else {
			buildError = eventBuilds.reason.message || String(eventBuilds.reason);
		}

		if (savegameBuildItems.status === 'fulfilled') {
			savegameBuilds = savegameBuildItems.value;
		} else {
			savegameBuildError = savegameBuildItems.reason.message || String(savegameBuildItems.reason);
		}

		if (eventSettings.status === 'fulfilled') {
			oldestBuild = eventSettings.value.OldestBuild || '';
		} else {
			settingsError = eventSettings.reason.message || String(eventSettings.reason);
		}

		loadingBuilds = false;
		loadingSavegameBuilds = false;
	}

	async function updateOldestBuild(event) {
		const previousBuild = oldestBuild;
		oldestBuild = event.currentTarget.value;
		savingOldestBuild = true;
		oldestBuildSaved = false;
		settingsError = '';

		try {
			const res = await fetch(`https://api.clyde.games/event/settings`, {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ OldestBuild: oldestBuild })
			});
			const settings = await parseResponse(res);
			if (!res.ok) {
				throw new Error(settings.error || settings);
			}
			oldestBuild = settings.OldestBuild || '';
			oldestBuildSaved = true;
		} catch (error) {
			oldestBuild = previousBuild;
			oldestBuildSaved = false;
			settingsError = error.message;
		} finally {
			savingOldestBuild = false;
		}
	}

	loadBuilds();

	async function refreshBuilds() {
		try {
			builds = await getBuilds();
			buildError = '';
		} catch (error) {
			buildError = error.message;
		}
	}

	async function refreshSavegameBuilds() {
		try {
			savegameBuilds = await getSavegameBuilds();
			savegameBuildError = '';
		} catch (error) {
			savegameBuildError = error.message;
		}
	}

	function formatBuildDate(build) {
		const date = new Date(build);

		if (isNaN(date.getTime())) {
			return build;
		}

		return date.toLocaleDateString(undefined, {
			year: 'numeric',
			month: 'short',
			day: 'numeric',
			hour: 'numeric',
			minute: '2-digit'
		});
	}

	function matchesProject(item) {
		return selectedProject === 'all' || (item.Project || '').toLowerCase() === selectedProject.toLowerCase();
	}

	function buildContext(build) {
		return build.Project || 'unknown';
	}

	async function deleteEventBuild(build) {
		const project = selectedProject === 'all' ? '' : build.Project;
		const projectLabel = project ? ` for ${project}` : '';
		if (!confirm(`Delete all metrics events for build ${build.Build}${projectLabel}?`)) {
			return;
		}

		deletingEventBuild = buildKey(build);
		deleteError = '';

		try {
			const projectParam = project ? `&project=${encodeURIComponent(project)}` : '';
			const res = await fetch(`https://api.clyde.games/event/build?build=${encodeURIComponent(build.Build)}${projectParam}`, {
				method: 'DELETE'
			});

			if (!res.ok) {
				const error = await parseResponse(res);
				deleteError = error.error || error;
				return;
			}

			builds = builds.filter(item => item.Build !== build.Build || (project && item.Project !== project));
			await refreshBuilds();
		} catch (error) {
			deleteError = error.message;
		} finally {
			deletingEventBuild = null;
		}
	}

	async function deleteSavegameBuildItems(build) {
		const project = selectedProject === 'all' ? '' : build.Project;
		const projectLabel = project ? ` for ${project}` : '';
		if (!confirm(`Delete all save games for build ${build.Build}${projectLabel}?`)) {
			return;
		}

		deletingSavegameBuild = buildKey(build);
		deleteError = '';

		try {
			const projectParam = project ? `&project=${encodeURIComponent(project)}` : '';
			const res = await fetch(`https://api.clyde.games/savegame/build?build=${encodeURIComponent(build.Build)}${projectParam}`, {
				method: 'DELETE'
			});

			if (!res.ok) {
				const error = await parseResponse(res);
				deleteError = error.error || error;
				return;
			}

			savegameBuilds = savegameBuilds.filter(item => item.Build !== build.Build || (project && item.Project !== project));
			await refreshSavegameBuilds();
		} catch (error) {
			deleteError = error.message;
		} finally {
			deletingSavegameBuild = null;
		}
	}

	function buildKey(build) {
		return `${build.Project || ''}:${build.Build}`;
	}

	$: visibleBuilds = builds.filter(matchesProject);
	$: visibleSavegameBuilds = savegameBuilds.filter(matchesProject);
	$: metricBuildDates = [...new Set([
		...builds.map(build => build.Build),
		...savegameBuilds.map(build => build.Build)
	])].sort().reverse();
	$: reportProjects('metrics', [
		...builds.map(build => build.Project),
		...savegameBuilds.map(build => build.Project)
	]);
</script>

<main>
	<header class="page-header">
		<div>
			<h2>Metrics</h2>
			<p>{visibleBuilds.length} event build{visibleBuilds.length === 1 ? '' : 's'} / {visibleSavegameBuilds.length} save build{visibleSavegameBuilds.length === 1 ? '' : 's'}</p>
		</div>
	</header>

	{#if deleteError}
		<p class="error">{deleteError}</p>
	{/if}

	<section class="settings">
		<div>
			<label for="oldest-build">Oldest accepted metric build</label>
			<p>Metric events and savegames from builds before this date will be rejected.</p>
		</div>
		<div class="setting-control">
			<select id="oldest-build" value={oldestBuild} disabled={loadingBuilds || savingOldestBuild} on:change={updateOldestBuild}>
				<option value="">Accept all builds</option>
				{#each metricBuildDates as build}
					<option value={build}>{formatBuildDate(build)}</option>
				{/each}
			</select>
			<div class="save-status" role="status" aria-live="polite">
				{#if savingOldestBuild}
					<span class="spinner" aria-hidden="true"></span>
					<span>Saving</span>
				{:else if oldestBuildSaved}
					<span class="saved-check" aria-hidden="true">✓</span>
					<span>Saved</span>
				{/if}
			</div>
		</div>
	</section>
	{#if settingsError}
		<p class="error">{settingsError}</p>
	{/if}

	<section class="builds">
		<div class="section-header">
			<h3>Event Builds</h3>
		</div>
		{#if loadingBuilds}
			<p class="state-message">loading...</p>
		{:else if buildError}
			<p class="state-message error">{buildError}</p>
		{:else}
			{#if visibleBuilds.length > 0}
				<div class="build-list">
					{#each visibleBuilds as build (buildKey(build))}
						<div class:deleting={deletingEventBuild === buildKey(build)} class="build-row">
							<p class="build-date">{formatBuildDate(build.Build)}</p>
							<p class="build-project">{buildContext(build)}</p>
							<p class="build-count">{build.Count} events</p>
							<button
								type="button"
								disabled={deletingEventBuild === buildKey(build)}
								on:click={() => deleteEventBuild(build)}
							>
								{deletingEventBuild === buildKey(build) ? 'Deleting...' : 'Delete All'}
							</button>
						</div>
					{/each}
				</div>
			{:else}
				<p class="empty-state">No builds found.</p>
			{/if}
		{/if}
	</section>

	<section class="builds">
		<div class="section-header">
			<h3>Save Game Builds</h3>
		</div>

		{#if loadingSavegameBuilds}
			<p class="state-message">loading...</p>
		{:else if savegameBuildError}
			<p class="state-message error">{savegameBuildError}</p>
		{:else}
			{#if visibleSavegameBuilds.length > 0}
				<div class="build-list">
					{#each visibleSavegameBuilds as build (buildKey(build))}
						<div class:deleting={deletingSavegameBuild === buildKey(build)} class="build-row">
							<p class="build-date">{formatBuildDate(build.Build)}</p>
							<p class="build-project">{buildContext(build)}</p>
							<p class="build-count">{build.Count} save game{build.Count === 1 ? '' : 's'}</p>
							<button
								type="button"
								disabled={deletingSavegameBuild === buildKey(build)}
								on:click={() => deleteSavegameBuildItems(build)}
							>
								{deletingSavegameBuild === buildKey(build) ? 'Deleting...' : 'Delete All'}
							</button>
						</div>
					{/each}
				</div>
			{:else}
				<p class="empty-state">No save game builds found.</p>
			{/if}
		{/if}
	</section>
</main>

<style>
	main {
		background: var(--surface);
		border: 1px solid var(--line);
		border-radius: var(--radius);
		color: var(--text);
		max-width: 1368px;
		margin: 12px auto 32px;
		padding: 14px;
	}

	.page-header {
		border-bottom: 1px solid var(--line);
		margin-bottom: 12px;
		padding-bottom: 10px;
	}

	.page-header h2 {
		font-size: 1.5rem;
		line-height: 1.15;
		text-align: left;
	}

	.page-header p {
		color: var(--text-muted);
		font-size: .82rem;
		font-weight: 400;
		letter-spacing: 0;
		margin: 3px 0 0;
		text-align: left;
	}

	.builds {
		background: var(--paper-warm);
		border-left: 3px solid var(--forest);
		border-radius: 3px;
		margin-top: 12px;
		padding: 4px 10px;
	}

	.settings {
		align-items: center;
		background: var(--paper-warm);
		border-left: 3px solid var(--forest);
		border-radius: 3px;
		display: flex;
		justify-content: space-between;
		gap: 20px;
		padding: 10px;
	}

	.settings label {
		font-weight: 800;
	}

	.settings p {
		color: var(--text-soft);
		font-size: 0.85rem;
		margin: 4px 0 0;
	}

	select {
		background: var(--surface);
		border: 1px solid var(--line-strong);
		border-radius: 4px;
		color: var(--text);
		font: inherit;
		min-width: 240px;
		padding: 7px 10px;
	}

	.setting-control {
		align-items: center;
		display: flex;
		gap: 10px;
	}

	.save-status {
		align-items: center;
		color: var(--text-soft);
		display: flex;
		font-size: 0.82rem;
		font-weight: 700;
		gap: 6px;
		min-width: 68px;
	}

	.spinner {
		animation: spin 700ms linear infinite;
		border: 2px solid var(--line-strong);
		border-radius: 50%;
		border-top-color: var(--forest);
		display: inline-block;
		height: 13px;
		width: 13px;
	}

	.saved-check {
		color: var(--forest);
		font-size: 1rem;
		line-height: 1;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}

	.section-header {
		border-bottom: 1px solid var(--line);
		padding: 12px 0;
	}

	.section-header h3 {
		font-size: 1rem;
		line-height: 1;
		margin: 0;
		text-align: left;
	}

	.build-list {
		display: flex;
		flex-direction: column;
	}

	.build-row {
		display: grid;
		grid-template-columns: minmax(150px, max-content) minmax(100px, max-content) minmax(120px, 1fr) auto;
		align-items: center;
		column-gap: 16px;
		border-top: 1px solid var(--line);
		padding: 12px 0;
		text-align: left;
		transition: opacity 120ms ease;
	}

	.section-header + .build-list .build-row:first-child {
		border-top: 0;
	}

	.build-row.deleting {
		opacity: 0.45;
		pointer-events: none;
	}

	.build-date,
	.build-project,
	.build-count {
		margin: 0;
		text-align: left;
	}

	.build-project {
		color: var(--text-muted);
		font-size: 0.82rem;
		font-weight: 800;
		overflow-wrap: anywhere;
	}

	.build-date {
		color: var(--charcoal);
		font-size: 0.95rem;
		font-weight: 800;
	}

	.build-count {
		color: var(--text-soft);
		font-size: 0.85rem;
		font-weight: 600;
		text-transform: uppercase;
	}

	button {
		background: var(--surface);
		border: 1px solid var(--line);
		border-radius: 4px;
		color: var(--text);
		cursor: pointer;
		font: inherit;
		font-size: .75rem;
		font-weight: 700;
		line-height: 1;
		padding: 6px 9px;
	}

	button:hover:not(:disabled) {
		background: var(--paper-warm);
		border-color: var(--charcoal);
	}

	button:disabled {
		color: var(--text-soft);
		cursor: default;
	}

	.empty-state {
		color: var(--text-soft);
		margin: 12px 0;
		text-align: left;
	}

	.error {
		color: var(--rust);
		margin: 12px 0;
		text-align: left;
	}

	.state-message {
		color: var(--text-soft);
		margin: 12px 0;
		text-align: left;
	}

	@media (max-width: 520px) {
		main {
			margin: 10px 8px 24px;
			padding: 14px;
		}

		.settings {
			align-items: stretch;
			flex-direction: column;
		}

		select {
			min-width: 0;
			width: 100%;
		}

		.setting-control {
			align-items: flex-start;
			flex-direction: column;
			gap: 6px;
		}

		.build-row {
			grid-template-columns: 1fr auto;
			row-gap: 4px;
		}

		.build-project {
			grid-column: 1;
			grid-row: 2;
		}

		.build-count {
			grid-column: 1;
			grid-row: 3;
		}

		button {
			grid-column: 2;
			grid-row: 1 / span 3;
		}
	}
</style>
