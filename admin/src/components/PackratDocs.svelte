<script>
	let rebuilding = false;
	let message = '';
	let error = '';
	let output = '';

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
	@media (max-width: 650px) { .packrat-docs { margin: 10px 8px; } header { align-items: stretch; flex-direction: column; } button { align-self: flex-start; } }
</style>
