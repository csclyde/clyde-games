<script>
	let users = [];
	let loading = true;
	let error = '';

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

	function formatDate(value) {
		const date = new Date(value);
		return isNaN(date.getTime()) ? value || '-' : date.toLocaleString();
	}

	loadUsers();
</script>

<section class="blocked-users" aria-labelledby="blocked-users-heading">
	<header>
		<div>
			<h2 id="blocked-users-heading">Blocked Users</h2>
			<p>Users whose metrics events and save games are rejected.</p>
		</div>
		<button class="refresh" type="button" on:click={loadUsers} disabled={loading}>{loading ? 'Refreshing…' : 'Refresh'}</button>
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
					<button type="button" on:click={() => unblock(user)}>Delete</button>
				</article>
			{/each}
		</div>
	{/if}
</section>

<style>
	.blocked-users { background: var(--surface); border: 1px solid var(--line); border-radius: var(--radius); color: var(--text); margin: 12px auto 32px; max-width: 1368px; padding: 14px; }
	header { align-items: flex-start; border-bottom: 1px solid var(--line); display: flex; gap: 8px; justify-content: space-between; margin-bottom: 12px; padding-bottom: 10px; }
	h2, header p { margin: 0; }
	h2 { font-size: 1.5rem; line-height: 1.15; text-align: left; }
	header p { color: var(--text-muted); font-size: .82rem; margin-top: 3px; text-align: left; }
	.users { display: flex; flex-direction: column; gap: 8px; }
	.users.scrolling { max-height: 360px; overflow-y: auto; padding-right: 5px; scrollbar-gutter: stable; }
	article { align-items: center; background: var(--paper-warm); border-left: 3px solid var(--rust); border-radius: 3px; display: flex; gap: 16px; justify-content: space-between; padding: 10px; }
	article div { display: flex; flex-direction: column; gap: 5px; min-width: 0; }
	a { color: var(--charcoal); font-weight: 800; overflow-wrap: anywhere; text-decoration: none; }
	a:hover { text-decoration: underline; }
	time { color: var(--text-soft); font-size: .75rem; }
	button { border: 1px solid var(--line); border-radius: 4px; cursor: pointer; font: inherit; font-size: .75rem; font-weight: 700; padding: 6px 9px; }
	button:not(.refresh) { background: var(--surface); border-color: var(--rust); color: var(--rust); }
	button:not(.refresh):hover { background: var(--rust); color: var(--surface); }
	button.refresh { background: var(--paper-warm); color: var(--text); }
	button.refresh:hover:not(:disabled) { border-color: var(--charcoal); }
	button:disabled { cursor: default; opacity: .6; }
	.state { color: var(--text-muted); margin: 6px 0; }
	.error { color: var(--rust); }
	@media (max-width: 650px) { .blocked-users { margin: 10px 8px 24px; } }
</style>
