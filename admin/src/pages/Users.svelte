<script>
	import { tick } from 'svelte';

	let users = [];
	let loading = true;
	let error = '';
	let addModalOpen = false;
	let newUserID = '';
	let adding = false;
	let addError = '';
	let userIDInput;

	async function loadUsers() {
		loading = true;
		error = '';
		try {
			const response = await fetch('https://api.clyde.games/users/blocked');
			const result = await response.json();
			if (!response.ok) throw new Error(result.error || result);
			users = result;
		} catch (loadError) {
			error = loadError.message;
		} finally {
			loading = false;
		}
	}

	async function unblock(user) {
		if (!confirm(`Remove the block for ${user.PID}?`)) return;
		error = '';
		try {
			const response = await fetch(`https://api.clyde.games/users/blocked/${encodeURIComponent(user.PID)}`, { method: 'DELETE' });
			const result = await response.json();
			if (!response.ok) throw new Error(result.error || result);
			users = users.filter(item => item.PID !== user.PID);
		} catch (deleteError) {
			error = deleteError.message;
		}
	}

	async function openAddModal() {
		newUserID = '';
		addError = '';
		addModalOpen = true;
		await tick();
		userIDInput.focus();
	}

	function closeAddModal() {
		if (adding) return;
		addModalOpen = false;
	}

	async function addUser() {
		const id = newUserID.trim();
		if (!id) {
			addError = 'User ID is required.';
			return;
		}

		adding = true;
		addError = '';
		try {
			const response = await fetch(`https://api.clyde.games/user/${encodeURIComponent(id)}/block`, { method: 'POST' });
			const result = await response.json();
			if (!response.ok) throw new Error(result.error || result);
			addModalOpen = false;
			await loadUsers();
		} catch (addUserError) {
			addError = addUserError.message;
		} finally {
			adding = false;
		}
	}

	function handleKeydown(event) {
		if (event.key === 'Escape' && addModalOpen) closeAddModal();
	}

	function formatDate(value) {
		const date = new Date(value);
		return isNaN(date.getTime()) ? value || '-' : date.toLocaleString();
	}

	loadUsers();
</script>

<svelte:window on:keydown={handleKeydown} />

<section class="blocked-users" aria-labelledby="blocked-users-heading">
	<header>
		<div>
			<h2 id="blocked-users-heading">Blocked Users</h2>
			<p class="summary">{loading ? 'Loading blocked users…' : `${users.length} blocked user${users.length === 1 ? '' : 's'}`}</p>
			<p>Users whose metrics events and save games are rejected.</p>
		</div>
		<div class="header-actions">
			<button class="add" type="button" on:click={openAddModal}>Add</button>
			<button class="refresh" type="button" on:click={loadUsers} disabled={loading}>{loading ? 'Refreshing…' : 'Refresh'}</button>
		</div>
	</header>

	{#if loading}
		<p class="state">loading...</p>
	{:else if error}
		<p class="state error">{error}</p>
	{:else if users.length === 0}
		<p class="state">No blocked users.</p>
	{:else}
		<div class="users" class:scrolling={users.length > 0}>
			{#each users as user}
				<article>
					<div><a href={`/user/${encodeURIComponent(user.PID)}`}>{user.PID}</a><time>Blocked {formatDate(user.CreatedAt)}</time></div>
					<button class="delete" type="button" on:click={() => unblock(user)}>Delete</button>
				</article>
			{/each}
		</div>
	{/if}
</section>

{#if addModalOpen}
	<div class="modal-backdrop" role="presentation" on:click|self={closeAddModal}>
		<form class="modal" role="dialog" aria-modal="true" aria-labelledby="add-user-heading" on:submit|preventDefault={addUser}>
			<h3 id="add-user-heading">Add Blocked User</h3>
			<label for="blocked-user-id">User ID</label>
			<input id="blocked-user-id" bind:this={userIDInput} bind:value={newUserID} maxlength="64" autocomplete="off" disabled={adding} />
			{#if addError}<p class="modal-error">{addError}</p>{/if}
			<div class="modal-actions">
				<button class="cancel" type="button" on:click={closeAddModal} disabled={adding}>Cancel</button>
				<button class="ok" type="submit" disabled={adding}>{adding ? 'Adding…' : 'OK'}</button>
			</div>
		</form>
	</div>
{/if}

<style>
	.blocked-users { background: var(--surface); border: 1px solid var(--line); border-radius: var(--radius); color: var(--text); margin: 12px auto 32px; max-width: 1368px; padding: 14px; }
	header { align-items: flex-start; border-bottom: 1px solid var(--line); display: flex; gap: 8px; justify-content: space-between; margin-bottom: 12px; padding-bottom: 10px; }
	h2, header p { margin: 0; }
	h2 { font-size: 1.5rem; line-height: 1.15; text-align: left; }
	header p { color: var(--text-muted); font-size: .82rem; margin-top: 3px; text-align: left; }
	header p.summary { color: var(--text); font-weight: 800; margin-top: 5px; }
	.header-actions { display: flex; gap: 8px; }
	.users { display: flex; flex-direction: column; gap: 8px; }
	.users.scrolling { max-height: 360px; overflow-y: auto; padding-right: 5px; scrollbar-gutter: stable; }
	article { align-items: center; background: var(--paper-warm); border-left: 3px solid var(--rust); border-radius: 3px; display: flex; gap: 16px; justify-content: space-between; padding: 10px; }
	article div { display: flex; flex-direction: column; gap: 5px; min-width: 0; }
	a { color: var(--charcoal); font-weight: 800; overflow-wrap: anywhere; text-decoration: none; }
	a:hover { text-decoration: underline; }
	time { color: var(--text-soft); font-size: .75rem; }
	button { border: 1px solid var(--line); border-radius: 4px; cursor: pointer; font: inherit; font-size: .75rem; font-weight: 700; padding: 6px 9px; }
	button.delete { background: var(--surface); border-color: var(--rust); color: var(--rust); }
	button.delete:hover { background: var(--rust); color: var(--surface); }
	button.refresh { background: var(--paper-warm); color: var(--text); }
	button.refresh:hover:not(:disabled) { border-color: var(--charcoal); }
	button.add, button.ok { background: var(--forest); border-color: var(--forest); color: var(--surface); }
	button.add:hover, button.ok:hover:not(:disabled) { filter: brightness(.9); }
	button.cancel { background: var(--paper-warm); color: var(--text); }
	button:disabled { cursor: default; opacity: .6; }
	.state { color: var(--text-muted); margin: 6px 0; }
	.error { color: var(--rust); }
	.modal-backdrop { align-items: center; background: rgba(20, 18, 14, .58); display: flex; inset: 0; justify-content: center; padding: 16px; position: fixed; z-index: 1000; }
	.modal { background: var(--surface); border: 1px solid var(--line); border-radius: var(--radius); box-shadow: 0 14px 40px rgba(0, 0, 0, .28); box-sizing: border-box; max-width: 440px; padding: 20px; width: 100%; }
	.modal h3 { color: var(--text); font-size: 1.25rem; margin: 0 0 18px; }
	.modal label { color: var(--text-soft); display: block; font-size: .75rem; font-weight: 800; letter-spacing: .04em; margin-bottom: 6px; text-transform: uppercase; }
	.modal input { background: var(--surface); border: 1px solid var(--line-strong); border-radius: 4px; box-sizing: border-box; color: var(--text); font: inherit; padding: 9px 10px; width: 100%; }
	.modal input:focus { border-color: var(--forest); outline: 2px solid var(--forest); outline-offset: 1px; }
	.modal-error { color: var(--rust); font-size: .82rem; margin: 8px 0 0; }
	.modal-actions { display: flex; gap: 8px; justify-content: flex-end; margin-top: 20px; }
	@media (max-width: 650px) { .blocked-users { margin: 10px 8px 24px; } }
</style>
