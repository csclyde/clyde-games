<script>
	export let selectedProject = 'Cursemark';
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

	let builds = [];
	let savegameBuilds = [];
	let loadingBuilds = true;
	let loadingSavegameBuilds = true;
	let buildError = '';
	let savegameBuildError = '';
	let deletingEventBuild = null;
	let deletingSavegameBuild = null;
	let deleteError = '';

	async function loadBuilds() {
		loadingBuilds = true;
		loadingSavegameBuilds = true;
		buildError = '';
		savegameBuildError = '';

		const [eventBuilds, savegameBuildItems] = await Promise.allSettled([getBuilds(), getSavegameBuilds()]);

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

		loadingBuilds = false;
		loadingSavegameBuilds = false;
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
		color: #171717;
		max-width: 1400px;
		margin: 0 auto;
		padding: 12px 16px 32px;
	}

	.page-header {
		border-bottom: 1px solid #d8d8d8;
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

	.builds {
		border: 1px solid #cfcfcf;
		border-left: 4px solid #111;
		border-radius: 4px;
		margin-top: 14px;
		padding: 4px 16px;
	}

	.section-header {
		border-bottom: 1px solid #ddd;
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
		border-top: 1px solid #ddd;
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
		color: #555;
		font-size: 0.82rem;
		font-weight: 800;
		overflow-wrap: anywhere;
	}

	.build-date {
		color: #111;
		font-size: 0.95rem;
		font-weight: 800;
	}

	.build-count {
		color: #666;
		font-size: 0.85rem;
		font-weight: 600;
		text-transform: uppercase;
	}

	button {
		background: #fff;
		border: 1px solid #a8a8a8;
		border-radius: 4px;
		color: #111;
		cursor: pointer;
		font: inherit;
		font-size: 0.82rem;
		font-weight: 700;
		line-height: 1;
		padding: 7px 10px;
	}

	button:hover:not(:disabled) {
		background: #f2f2f2;
		border-color: #666;
	}

	button:disabled {
		color: #777;
		cursor: default;
	}

	.empty-state {
		color: #666;
		margin: 12px 0;
		text-align: left;
	}

	.error {
		color: #a40000;
		margin: 12px 0;
		text-align: left;
	}

	.state-message {
		color: #666;
		margin: 12px 0;
		text-align: left;
	}

	@media (max-width: 520px) {
		main {
			padding: 8px;
		}

		.page-header h2 {
			font-size: 2rem;
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
