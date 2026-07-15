<script>
	import { createEventDispatcher } from 'svelte';

	export let project = null;

	const dispatch = createEventDispatcher();
	let name = '';
	let steamAppId = '';
	let subredditUrl = '';
	let saving = false;
	let error = '';
	let loadedProjectId = null;

	$: if (!project) {
		loadedProjectId = null;
	} else if (loadedProjectId !== project.id) {
		name = project.name;
		steamAppId = project.steamAppId || '';
		subredditUrl = project.subredditUrl || '';
		error = '';
		loadedProjectId = project.id;
	}

	function close() {
		if (saving) return;
		discardDraft();
		dispatch('close');
	}

	function discardDraft() {
		name = '';
		steamAppId = '';
		subredditUrl = '';
		error = '';
		loadedProjectId = null;
	}

	async function save() {
		const trimmedName = name.trim();
		if (!trimmedName) {
			error = 'Project name is required.';
			return;
		}

		saving = true;
		error = '';
		try {
			const response = await fetch(`https://api.clyde.games/projects/${encodeURIComponent(project.id)}`, {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					name: trimmedName,
					steamAppId: steamAppId.trim(),
					subredditUrl: subredditUrl.trim()
				})
			});
			const body = await response.json();
			if (!response.ok) throw new Error(body.error || 'Unable to update project');
			dispatch('saved', body);
			discardDraft();
			dispatch('close');
		} catch (saveError) {
			error = saveError.message || 'Unable to update project';
		} finally {
			saving = false;
		}
	}
</script>

{#if project}
	<!-- svelte-ignore a11y-click-events-have-key-events -->
	<div class="modal-backdrop" role="presentation" on:click|self={close}>
		<form class="modal" role="dialog" aria-modal="true" aria-labelledby="project-modal-title" on:submit|preventDefault={save}>
			<h3 id="project-modal-title">Edit project</h3>
			<p class="project-id">ID: <code>{project.id}</code></p>
			<label for="project-name">Name</label>
			<input id="project-name" bind:value={name} maxlength="255" autocomplete="off" disabled={saving} />
			<label for="steam-app-id">Steam app ID</label>
			<input id="steam-app-id" bind:value={steamAppId} maxlength="32" inputmode="numeric" autocomplete="off" placeholder="3219180" disabled={saving} />
			<label for="subreddit-url">Subreddit URL</label>
			<input id="subreddit-url" bind:value={subredditUrl} maxlength="512" type="url" autocomplete="off" placeholder="https://www.reddit.com/r/cursemark" disabled={saving} />
			<p class="source-help">Leave a source blank to skip it when this project is synced.</p>
			{#if error}<p class="modal-error">{error}</p>{/if}
			<div class="modal-actions">
				<button type="button" on:click={close} disabled={saving}>Cancel</button>
				<button type="submit" class="primary" disabled={saving || !name.trim()}>{saving ? 'Saving…' : 'Confirm changes'}</button>
			</div>
		</form>
	</div>
{/if}

<style>
	.modal-backdrop { align-items: center; background: rgba(20, 18, 14, .58); display: flex; inset: 0; justify-content: center; padding: 16px; position: fixed; z-index: 1000; }
	.modal { background: var(--surface); border: 1px solid var(--line); border-radius: var(--radius); box-shadow: 0 14px 40px rgba(0, 0, 0, .28); box-sizing: border-box; max-width: 440px; padding: 20px; width: 100%; }
	h3 { color: var(--text); font-size: 1.25rem; margin: 0; }
	.project-id { color: var(--text-muted); font-size: .82rem; margin: 6px 0 18px; }
	code { color: var(--text); }
	label { color: var(--text-soft); display: block; font-size: .75rem; font-weight: 800; letter-spacing: .04em; margin: 14px 0 6px; text-transform: uppercase; }
	input { background: var(--surface); border: 1px solid var(--line-strong); border-radius: 4px; box-sizing: border-box; color: var(--text); font: inherit; padding: 9px 10px; width: 100%; }
	input:focus { border-color: var(--forest); outline: 2px solid var(--forest); outline-offset: 1px; }
	.source-help { color: var(--text-muted); font-size: .76rem; margin: 10px 0 0; }
	.modal-error { color: var(--rust); font-size: .82rem; margin: 8px 0 0; }
	.modal-actions { display: flex; gap: 8px; justify-content: flex-end; margin-top: 20px; }
	button { background: var(--paper-warm); border: 1px solid var(--line); border-radius: 4px; color: var(--text); cursor: pointer; font: inherit; font-size: .78rem; font-weight: 700; padding: 7px 11px; }
	button.primary { background: var(--forest); border-color: var(--forest); color: white; }
	button:disabled { cursor: default; opacity: .6; }
</style>
