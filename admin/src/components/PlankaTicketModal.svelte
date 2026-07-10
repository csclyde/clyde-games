<script>
	import { createEventDispatcher } from 'svelte';

	export let open = false;
	export let title = '';
	export let description = '';

	const dispatch = createEventDispatcher();

	let plankaProjects = [];
	let selectedPlankaProject = '';
	let selectedPlankaBoard = '';
	let selectedPlankaList = '';
	let loadingPlanka = false;
	let creatingTicket = false;
	let plankaError = '';

	$: plankaBoards = (plankaProjects.find(project => project.id === selectedPlankaProject) || {}).boards || [];
	$: plankaLists = (plankaBoards.find(board => board.id === selectedPlankaBoard) || {}).lists || [];

	$: if (open) {
		loadPlankaHierarchy();
	}

	async function loadPlankaHierarchy() {
		plankaError = '';
		if (plankaProjects.length || loadingPlanka) {
			return;
		}

		loadingPlanka = true;
		try {
			const res = await fetch('https://api.clyde.games/planka/hierarchy');
			const result = await res.json();
			if (!res.ok) {
				throw new Error(result.error || 'Unable to load Planka');
			}

			plankaProjects = result;
			selectedPlankaProject = result[0] ? result[0].id : '';
			selectedPlankaBoard = result[0] && result[0].boards[0] ? result[0].boards[0].id : '';
			selectedPlankaList = result[0] && result[0].boards[0] && result[0].boards[0].lists[0] ? result[0].boards[0].lists[0].id : '';
		} catch (error) {
			plankaError = error.message;
		} finally {
			loadingPlanka = false;
		}
	}

	function closeModal() {
		if (creatingTicket) {
			return;
		}

		plankaError = '';
		dispatch('close');
	}

	function changePlankaProject() {
		selectedPlankaBoard = plankaBoards[0] ? plankaBoards[0].id : '';
		selectedPlankaList = plankaBoards[0] && plankaBoards[0].lists[0] ? plankaBoards[0].lists[0].id : '';
	}

	function changePlankaBoard() {
		selectedPlankaList = plankaLists[0] ? plankaLists[0].id : '';
	}

	async function createTicket() {
		creatingTicket = true;
		plankaError = '';

		try {
			const res = await fetch('https://api.clyde.games/planka/tickets', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					listId: selectedPlankaList,
					name: title,
					description
				})
			});
			const result = await res.json();
			if (!res.ok) {
				throw new Error(result.error || 'Unable to create ticket');
			}

			dispatch('created', result);
			dispatch('close');
		} catch (error) {
			plankaError = error.message;
		} finally {
			creatingTicket = false;
		}
	}
</script>

{#if open}
	<!-- svelte-ignore a11y-click-events-have-key-events -->
	<div class="modal-backdrop" role="presentation" on:click|self={closeModal}>
		<section class="ticket-modal" role="dialog" aria-modal="true" aria-labelledby="ticket-modal-title">
			<h3 id="ticket-modal-title">Make Planka Ticket</h3>
			<p class="ticket-preview">{title}</p>
			{#if loadingPlanka}
				<p>Loading Planka projects...</p>
			{:else}
				<label>Project<select bind:value={selectedPlankaProject} on:change={changePlankaProject} disabled={creatingTicket}>{#each plankaProjects as project}<option value={project.id}>{project.name}</option>{/each}</select></label>
				<label>Board<select bind:value={selectedPlankaBoard} on:change={changePlankaBoard} disabled={creatingTicket || !selectedPlankaProject}>{#each plankaBoards as board}<option value={board.id}>{board.name}</option>{/each}</select></label>
				<label>List<select bind:value={selectedPlankaList} disabled={creatingTicket || !selectedPlankaBoard}>{#each plankaLists as list}<option value={list.id}>{list.name}</option>{/each}</select></label>
			{/if}
			{#if plankaError}<small class="helper-error">{plankaError}</small>{/if}
			<div class="modal-actions">
				<button type="button" disabled={creatingTicket} on:click={closeModal}>Cancel</button>
				<button type="button" disabled={loadingPlanka || creatingTicket || !selectedPlankaList} on:click={createTicket}>{creatingTicket ? 'Creating...' : 'Create Ticket'}</button>
			</div>
		</section>
	</div>
{/if}

<style>
	.modal-backdrop { align-items: center; background: rgba(25, 28, 25, 0.65); display: flex; inset: 0; justify-content: center; padding: 16px; position: fixed; z-index: 1000; }
	.ticket-modal { background: var(--surface); border: 1px solid var(--line-strong); border-radius: var(--radius); box-shadow: 0 18px 60px rgba(0,0,0,.3); box-sizing: border-box; display: flex; flex-direction: column; gap: 14px; max-width: 520px; padding: 22px; width: 100%; }
	.ticket-modal h3, .ticket-modal p { margin: 0; text-align: left; }
	.ticket-preview { color: var(--text-muted); overflow-wrap: anywhere; }
	.ticket-modal label { color: var(--text-soft); display: flex; flex-direction: column; font-size: .72rem; font-weight: 800; gap: 5px; letter-spacing: .06em; text-transform: uppercase; }
	.ticket-modal select { background: var(--surface); border: 1px solid var(--line-strong); border-radius: 4px; color: var(--text); font: inherit; padding: 9px; }
	.modal-actions { display: flex; gap: 8px; justify-content: flex-end; margin-top: 4px; }
	.helper-error { color: var(--rust); max-width: 520px; }
	small { font-size: 0.68rem; line-height: 1.3; overflow-wrap: anywhere; text-align: left; }
</style>
