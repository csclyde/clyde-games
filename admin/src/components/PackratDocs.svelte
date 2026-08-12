<script>
	let rebuilding = false;
	let message = '';
	let error = '';
	let output = '';
	let loadingVersions = true;
	let savingVersion = false;
	let versionMessage = '';
	let versionError = '';
	let latestVersion = '0.1.0';
	let observedVersions = [];

	async function parseResponse(response) {
		const contentType = response.headers.get('content-type') || '';
		if (!contentType.includes('application/json')) {
			throw new Error('Packrat version API returned a non-JSON response');
		}
		const body = await response.json();
		if (!response.ok) throw new Error(body.error || body);
		return body;
	}

	async function loadVersionSettings() {
		loadingVersions = true;
		versionError = '';
		try {
			const settings = await parseResponse(await fetch('https://api.clyde.games/packrat/version/settings'));
			latestVersion = settings.LatestVersion || '0.1.0';
			observedVersions = settings.ObservedVersions || [];
		} catch (loadError) {
			versionError = loadError.message || 'Packrat version settings failed to load';
		} finally {
			loadingVersions = false;
		}
	}

	async function updateLatestVersion() {
		savingVersion = true;
		versionMessage = '';
		versionError = '';
		try {
			const settings = await parseResponse(await fetch('https://api.clyde.games/packrat/version/settings', {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ LatestVersion: latestVersion })
			}));
			latestVersion = settings.LatestVersion || latestVersion;
			observedVersions = settings.ObservedVersions || observedVersions;
			versionMessage = 'Saved';
		} catch (saveError) {
			versionError = saveError.message || 'Packrat version settings failed to save';
		} finally {
			savingVersion = false;
		}
	}

	async function rebuildDocs() {
		rebuilding = true;
		message = '';
		error = '';
		output = '';

		try {
			const response = await fetch('/api/packrat/rebuild', { method: 'POST' });
			const body = await response.json();
			if (!response.ok) throw new Error(body.error || 'Packrat documentation deployment failed');
			message = body.message || 'Packrat documentation deployed';
			output = body.output || '';
		} catch (rebuildError) {
			error = rebuildError.message || 'Packrat documentation deployment failed';
		} finally {
			rebuilding = false;
		}
	}

	loadVersionSettings();
</script>

<section class="packrat-docs" aria-labelledby="packrat-docs-heading">
	<header>
		<div>
			<h2 id="packrat-docs-heading">Packrat</h2>
			<p>Pull the latest docs, build VitePress, and publish them to packrat.clyde.games.</p>
		</div>
		<button type="button" on:click={rebuildDocs} disabled={rebuilding}>
			{rebuilding ? 'Rebuilding…' : 'Rebuild Docs'}
		</button>
	</header>

	{#if message}<p class="message success">{message}</p>{/if}
	{#if error}<p class="message error">{error}</p>{/if}
	{#if output}<pre>{output}</pre>{/if}

	<div class="version-settings">
		<div>
			<label for="packrat-latest-version">Latest app version</label>
			<p>Packrat users on older versions will see the update notice on startup.</p>
		</div>
		<div class="version-control">
			<select
				id="packrat-latest-version"
				bind:value={latestVersion}
				disabled={loadingVersions || savingVersion}
				on:change={updateLatestVersion}
			>
				{#if observedVersions.length === 0}
					<option value="0.1.0">0.1.0</option>
				{:else}
					{#each observedVersions as version}
						<option value={version}>{version}</option>
					{/each}
				{/if}
			</select>
			<div class="save-status" role="status" aria-live="polite">
				{#if loadingVersions}
					<span>Loading</span>
				{:else if savingVersion}
					<span>Saving</span>
				{:else if versionMessage}
					<span>{versionMessage}</span>
				{/if}
			</div>
		</div>
	</div>
	{#if versionError}<p class="message error">{versionError}</p>{/if}
</section>

<style>
	.packrat-docs { background: var(--surface); border: 1px solid var(--line); border-radius: var(--radius); margin: 12px auto; max-width: 1368px; padding: 14px; }
	header { align-items: flex-start; border-bottom: 1px solid var(--line); display: flex; gap: 16px; justify-content: space-between; padding-bottom: 10px; }
	h2 { font-size: 1.5rem; line-height: 1.15; margin: 0; text-align: left; }
	header p { color: var(--text-muted); font-size: .82rem; margin: 3px 0 0; text-align: left; }
	button { background: var(--charcoal); border: 1px solid var(--charcoal); border-radius: 4px; color: var(--surface); cursor: pointer; font: inherit; font-size: .75rem; font-weight: 800; padding: 8px 11px; white-space: nowrap; }
	button:hover { background: var(--olive); border-color: var(--olive); }
	button:disabled { cursor: wait; opacity: .6; }
	.message { font-size: .82rem; margin: 10px 0 0; }
	.success { color: var(--olive); }
	.error { color: #a13c34; }
	pre { background: var(--paper-warm); border: 1px solid var(--line); border-radius: 3px; color: var(--text-muted); font-size: .72rem; margin: 10px 0 0; max-height: 260px; overflow: auto; padding: 10px; white-space: pre-wrap; }
	.version-settings { align-items: center; border-top: 1px solid var(--line); display: flex; gap: 16px; justify-content: space-between; margin-top: 12px; padding-top: 12px; }
	.version-settings label { display: block; font-size: .78rem; font-weight: 800; margin-bottom: 3px; text-transform: uppercase; }
	.version-settings p { color: var(--text-muted); font-size: .82rem; margin: 0; }
	.version-control { align-items: center; display: flex; gap: 10px; }
	select { background: var(--surface); border: 1px solid var(--line); border-radius: 4px; color: var(--text); font: inherit; min-width: 150px; padding: 8px 10px; }
	.save-status { color: var(--text-muted); font-size: .78rem; min-width: 56px; }
	@media (max-width: 650px) { .packrat-docs { margin: 10px 8px; } header, .version-settings { align-items: stretch; flex-direction: column; } button { align-self: flex-start; } .version-control { align-items: stretch; flex-direction: column; } }
</style>
