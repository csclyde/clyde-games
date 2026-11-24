<script>
 import {canWrite} from "../portal-auth.mjs";
 import {parseCrashLog} from '../crash-runtime.mjs';
 import CrashBuildChart from '../components/CrashBuildChart.svelte';
 import CrashInvestigation from '../components/CrashInvestigation.svelte';
	import PlankaTicketModal from '../components/PlankaTicketModal.svelte';
	import Panel from '../components/Panel.svelte';
 import CrashHealth from '../components/CrashHealth.svelte';
	import { groupCrashList, crashResolutionIDs } from '../crash-groups.mjs';

	export let selectedProject = 'all';
	export let reportProjects = () => {};
 export let reportCrashCounts = () => {};

	async function parseResponse(res) {
		const text = await res.text();

		try {
			return JSON.parse(text);
		} catch (error) {
			if (res.ok) {
				throw new Error('Expected JSON response from crash API');
			}

			throw new Error(text || res.statusText);
		}
	}

	async function getCrashes() {
		const res = await fetch(`/api/crash`);
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


	async function getCrashBuilds() {
		const res = await fetch(`/api/crash/builds?project=${encodeURIComponent(selectedProject)}`);
		const builds = await parseResponse(res);

		if (res.ok) {
			return builds;
		} else {
			throw new Error(builds.error || builds);
		}
	}

	async function getCrashSettings() {
		const res = await fetch(`/api/crash/settings?project=${encodeURIComponent(selectedProject)}`);
		const settings = await parseResponse(res);

		if (res.ok) {
			return settings;
		} else {
			throw new Error(settings.error || settings);
		}
	}

	let queueProject=null;
 let health=null,healthRefresh=0,queueSearch='',queueView='all',queueSort='recent',buildFilter=null;
 let crashes = [];
	let crashBuilds = [];
	let loadingCrashes = true;
	let loadedCrashSettingsProject = null;
 let loadingCrashSettings = true;
	let crashError = '';
	let crashSettingsError = '';
	let resolvingCrashes = {};
	let resolvingAllCrashes = false;
	let helperStatus = {};
	let ticketCrash = null;
	let oldestCrashBuild = '';
	let savingOldestCrashBuild = false;
	let oldestCrashBuildSaved = false;

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


	async function loadCrashSettings() {
 const scope=selectedProject;
 if(scope==='all'){crashBuilds=[];oldestCrashBuild='';loadingCrashSettings=false;return}
		loadingCrashSettings = true;
		crashSettingsError = '';

		const [buildItems, settings] = await Promise.allSettled([
			getCrashBuilds(),
			getCrashSettings()
		]);

		if(scope!==selectedProject)return;
 if (buildItems.status === 'fulfilled') {
			crashBuilds = buildItems.value;
		} else {
			crashSettingsError = buildItems.reason.message || String(buildItems.reason);
		}

		if (settings.status === 'fulfilled') {
			oldestCrashBuild = settings.value.OldestBuild || '';
		} else {
			crashSettingsError = settings.reason.message || String(settings.reason);
		}

		loadingCrashSettings = false;
	}

	loadCrashes();

	async function refreshCrashLists() {
		try {
			crashes = await getCrashes();
 healthRefresh++;
			crashError = '';
		} catch (error) {
			crashError = error.message;
		}

	}

	async function updateOldestCrashBuild(event) {
		const scope=selectedProject;
 if(scope==='all')return;
 const previousBuild = oldestCrashBuild;
		oldestCrashBuild = event.currentTarget.value;
		savingOldestCrashBuild = true;
		oldestCrashBuildSaved = false;
		crashSettingsError = '';

		try {
			const res = await fetch(`/api/crash/settings?project=${encodeURIComponent(scope)}`, {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ OldestBuild: oldestCrashBuild })
			});
			const settings = await parseResponse(res);
 if(scope!==selectedProject)return;
			if (!res.ok) {
				throw new Error(settings.error || settings);
			}
			if(scope!==selectedProject)return;
 oldestCrashBuild = settings.OldestBuild || '';
			oldestCrashBuildSaved = true;
		} catch (error) {
			if(scope!==selectedProject)return;
 oldestCrashBuild = previousBuild;
			oldestCrashBuildSaved = false;
			crashSettingsError = error.message;
		} finally {
			savingOldestCrashBuild = false;
		}
	}


	function getFirstStackLine(crash) {
		const stack = crash.IsAccessViolationFamily ? [] : parseCrashLog(crash).frames;
		return stack.length > 0 && stack[0].trim() ? stack[0].trim() : 'Unknown location';
	}








	function getRegularCrashes(crashes) {
		return groupCrashList(crashes);
	}

	function getCrashKey(crash) {
		return crash.ListKey || crash.ID || crash.Hash || `${crash.Message || ''}:${crash.Stack || ''}`;
	}

	function getVisibleStack(crash) {
		const stack = crash.IsAccessViolationFamily ? [] : parseCrashLog(crash).frames;

		if (crash.expanded) {
			return stack;
		}

		return stack.slice(0, 6);
	}

	function shouldShowExpand(crash) {
		return !crash.IsAccessViolationFamily && parseCrashLog(crash).frames.length > 6;
	}

	function toggleCrashExpansion(crash) {
		const crashKey = getCrashKey(crash);
		crashes = crashes.map(item => getCrashKey(item) === crashKey
			? { ...item, expanded: !item.expanded }
			: item);
	}

	function formatDate(value) {
		if (!value) return '-';
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

	function matchesProject(item, project) {
		return project === 'all' || (item.Project || '').toLowerCase() === project.toLowerCase();
	}

	async function resolveCrash(crash) {
		const key = getCrashKey(crash);
		resolvingCrashes = { ...resolvingCrashes, [key]: true };

		try {
   const results = await Promise.all(crashResolutionIDs(crash).map(id => fetch(`/api/resolvecrash?id=${id}`, {method:'POST'})));
   if (results.some(res => !res.ok)) throw new Error('Some reports could not be resolved. Remaining reports are shown below.');
			await refreshCrashLists();
		} catch (error) {
   await refreshCrashLists();
			crashError = error.message;
		} finally {
			const nextResolvingCrashes = { ...resolvingCrashes };
			delete nextResolvingCrashes[key];
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
			`Report IDs: ${(crash.Reports || [crash]).map(report => report.ID).join(', ')}`,
			`Group: ${crash.GroupKey || 'unknown'}`,
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
			`Client PID: ${metadataValue(crash.PID)}`,
			`Count: ${metadataValue(crash.Count)}`,
			`Last ingested: ${formatDate(crash.LastIngestedAt || crash.IngestedAt)}`,
			'',
			'Stack:',
			...parseCrashLog(crash).frames,
			...parseCrashLog(crash, {includeGuards:true}).details.map(detail => `${detail.label}: ${detail.value}`)
		];

		if (crash.IsAccessViolationFamily) details.push('Individual reports:', ...crash.Reports.map(r => `Report #${r.ID} | group ${r.GroupKey || 'legacy'} | build ${r.Build || 'unknown'} | player ${r.PID || 'unknown'}\n${r.Message}\n${r.Stack}`));
		return details.join('\n');
	}

	async function handleTicketCreated() {
		const crash = ticketCrash;
		if (!crash) {
			return;
		}

		helperStatus = { ...helperStatus, [getCrashKey(crash)]: { status: 'Planka ticket created', error: '' } };
		ticketCrash = null;
		// Ticket creation retains all investigation material. Resolution is an
		// explicit, separate action, consistent with the MCP workflow.
	}

	async function resolveAllCrashes() {
		resolvingAllCrashes = true;

		try {
			const crashesToResolve = [...new Set(visibleRegularCrashes.flatMap(crashResolutionIDs))];
			const results = await Promise.all(crashesToResolve.map(id => fetch(`/api/resolvecrash?id=${id}`, { method: 'POST' })));
			if (results.some(result => !result.ok)) throw new Error('Some crash groups could not be resolved. Refresh to review remaining reports.');
			await refreshCrashLists();
		} catch (error) {
			await refreshCrashLists();
			crashError = error.message;
		} finally {
			resolvingAllCrashes = false;
		}
	}


 function healthFor(crash,data){if(!data)return null;const raw=(crash.Project||'').trim();const project=raw.toLowerCase()==='cursemark'?'cursemark':raw;if(crash.IsAccessViolationFamily)return data.families.find(g=>g.project===project);return data.groups.find(g=>g.project===project&&(crash.GroupKey?g.group_key===crash.GroupKey:g.report_id===crash.ID))}
 function filterIssueQueue(groups,query,view,sort,filter,data){const q=query.trim().toLowerCase();const out=groups.filter(g=>{const h=healthFor(g,data);if(q&&!g.Reports.some(r=>[r.Message,r.Stack,r.PID,r.Build].some(v=>(v||'').toLowerCase().includes(q))))return false;if(filter&&!g.Reports.some(r=>matchesProject(r,filter.project)&&r.Build===filter.build))return false;if(view==='returned'&&!h?.returned_open)return false;if(view==='detailed'&&!g.Reports.some(r=>r.OccurrenceVersion===1))return false;if(view==='legacy'&&g.Reports.some(r=>r.OccurrenceVersion===1))return false;return true});return out.sort((a,b)=>{const x=healthFor(a,data),y=healthFor(b,data);if(sort==='reports'&&(x?.reports||0)!==(y?.reports||0))return(y?.reports||0)-(x?.reports||0);if(sort==='ids'&&(x?.reporting_ids||0)!==(y?.reporting_ids||0))return(y?.reporting_ids||0)-(x?.reporting_ids||0);return (Date.parse(y?.last_received||b.LastIngestedAt)||0)-(Date.parse(x?.last_received||a.LastIngestedAt)||0)||b.ID-a.ID})}
	$: if(selectedProject!==loadedCrashSettingsProject){loadedCrashSettingsProject=selectedProject;loadCrashSettings()}
 $: visibleCrashes = crashes.filter(item => matchesProject(item, selectedProject));
	$: allRegularCrashes = getRegularCrashes(visibleCrashes);
 $: visibleRegularCrashes = filterIssueQueue(allRegularCrashes,queueSearch,queueView,queueSort,buildFilter,health);
 $: if(selectedProject!==queueProject){queueProject=selectedProject;buildFilter=null;queueSearch=''}
	$: crashBuildDates = [...new Set(crashBuilds.map(build => build.Build))].sort().reverse();
	$: if(!loadingCrashes && !crashError) reportCrashCounts(crashes);
	$: reportProjects('crashes', [
		...crashes.map(crash => crash.Project)
	]);
</script>

<main>



 <div class="queue-heading" id="crash-queue"></div>
 <div class="incident-overview"><div><span>Open incidents</span><b>{loadingCrashes ? '…' : new Set(visibleCrashes.map(r=>`${r.Project}:${r.GroupKey||r.ID}`)).size}</b></div><div><span>Stored reports</span><b>{loadingCrashes ? '…' : visibleCrashes.length}</b></div><div><span>Returned incidents</span><b>{health?.returned_open_groups ?? '—'}</b></div></div>
 <div class="queue-controls">
  <label class="search-control"><span>Search issues</span><input aria-label="Search crash issues" bind:value={queueSearch} placeholder="Error, stack, build or client ID"/></label>
  <label><span>Show</span><select aria-label="Crash issue view" bind:value={queueView}><option value="all">All open issues</option><option value="returned">Returned after resolution</option><option value="detailed">With detailed reports</option><option value="legacy">Legacy only</option></select></label>
  <label><span>Sort</span><select aria-label="Crash issue sort" bind:value={queueSort}><option value="recent">Latest detailed receipt</option><option value="reports">Recent report count</option><option value="ids">Recent reporting IDs</option></select></label>
			<label class="build-cutoff-control" for="oldest-crash-build">
				<span>Oldest Build</span>
				<select id="oldest-crash-build" value={oldestCrashBuild} disabled={!$canWrite || selectedProject === 'all' || loadingCrashSettings || savingOldestCrashBuild} on:change={updateOldestCrashBuild}>
					<option value="">Accept all builds</option>
 {#if oldestCrashBuild && !crashBuildDates.includes(oldestCrashBuild)}<option value={oldestCrashBuild}>{formatBuildDate(oldestCrashBuild)}</option>{/if}
					{#each crashBuildDates as build}
						<option value={build}>{formatBuildDate(build)}</option>
					{/each}
				</select>
			</label>
			<div class="save-status" role="status" aria-live="polite">
				{#if savingOldestCrashBuild}
					<span class="spinner" aria-hidden="true"></span>
					<span>Saving</span>
				{:else if oldestCrashBuildSaved}
					<span class="saved-check" aria-hidden="true">✓</span>
					<span>Saved</span>
				{/if}
			</div>
			<button type="button" disabled={!$canWrite || resolvingAllCrashes || visibleRegularCrashes.length === 0} on:click={resolveAllCrashes}>
				{resolvingAllCrashes ? 'Closing...' : 'Close All'}
			</button>
 </div>
 {#if buildFilter}<p class="build-filter">Open issues with build <b>{buildFilter.build||'Unknown'}</b> in {buildFilter.project||'untagged clients'} <button on:click={()=>buildFilter=null}>Clear build filter</button></p>{/if}
	{#if crashSettingsError}
		<p class="state-message error">{crashSettingsError}</p>
	{/if}


	{#if crashError}<p class="state-message error" role="alert">{crashError}</p>{/if}
	{#if loadingCrashes}
		<p class="state-message">loading...</p>
	{:else if queueView==='returned'&&!health}<p class="state-message">Complete returned-issue data is unavailable. Choose all open issues to review the queue.</p>
	{:else if visibleRegularCrashes.length === 0}
		<p class="state-message">{allRegularCrashes.length ? 'No open issues match these filters.' : 'No open crashes.'}</p>
	{:else}
		<div class="comment-list">
		{#each visibleRegularCrashes as crash (getCrashKey(crash))}
			<Panel inset={false} class={resolvingCrashes[getCrashKey(crash)] || resolvingAllCrashes ? "comment resolving" : "comment"}>
				<div class="comment-body">
					<div class="comment-main">
						<div class="stack">
							{#if crash.LegacyReportCount}<div class="issue-state"><span class="legacy-badge">Legacy history</span></div>{/if}
 <h3 class="message">{crash.Message}</h3>
 {#each getVisibleStack(crash) as stackMessage, index}
								<p class="stack-frame"><span>{index + 1}</span>{stackMessage}</p>
							{/each}
							{#if shouldShowExpand(crash)}
								<button class="expand-button" type="button" on:click={() => toggleCrashExpansion(crash)}>
									{crash.expanded ? 'Collapse stack trace' : 'Expand stack trace'}
								</button>
							{/if}

       <CrashBuildChart reports={crash.Reports}/>
       <section class="crash-details" aria-label="Crash runtime and incident details">
        <dl>
         {#if !crash.IsAccessViolationFamily}
          {#each parseCrashLog(crash).details as detail}<div><dt>{detail.label}</dt><dd>{detail.value}</dd></div>{/each}
          <div><dt>Build</dt><dd>{formatBuildDate(crash.Build)}</dd></div>
         {/if}
         <div><dt>Recent reports</dt><dd title={`Detailed receipts in the selected ${health?.days || 30}-day analysis window`}>{healthFor(crash,health)?.reports ?? 'Unavailable'}</dd></div>
         <div><dt>Reporting IDs</dt><dd title="Distinct installation identifiers in the selected analysis window; not verified people">{healthFor(crash,health)?.reporting_ids ?? 'Unavailable'}</dd></div>
         <div><dt>First received</dt><dd title="Earliest retained detailed receipt, across all time; legacy timestamps are unknown">{healthFor(crash,health)?.first_received ? formatDate(healthFor(crash,health).first_received) : (!health ? 'Unavailable' : 'Unknown / legacy')}</dd></div>
         {#if healthFor(crash,health)?.last_resolution}<div><dt>Recorded resolution</dt><dd>{formatDate(healthFor(crash,health).last_resolution)}</dd></div>{/if}
         <div><dt>Open occurrences</dt><dd title="Includes aggregate counts from legacy reports">{crash.Count}</dd></div>
        </dl>
       </section>
       <CrashInvestigation id={crash.ID} project={crash.Project} family={crash.IsAccessViolationFamily ? 'access_violations' : ''}/>

							{#if helperStatus[getCrashKey(crash)] && helperStatus[getCrashKey(crash)].status}
								<small class="helper-status">{helperStatus[getCrashKey(crash)].status}</small>
							{/if}
							{#if helperStatus[getCrashKey(crash)] && helperStatus[getCrashKey(crash)].error}
								<small class="helper-error">{helperStatus[getCrashKey(crash)].error}</small>
							{/if}
						</div>
						{#if !crash.IsAccessViolationFamily}<div class="meta-list">
							{#if selectedProject === 'all' && crash.Project}<span class="project-tag">project: {crash.Project}</span>{/if}
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
						</div>{/if}
					</div>
					<div class="comment-side">
      <div class="stats primary-stats"><p><span>Reports</span>{crash.Reports.length}</p><p><span>Last received</span>{healthFor(crash,health)?.last_received || crash.LastIngestedAt ? formatDate(healthFor(crash,health)?.last_received || crash.LastIngestedAt) : 'Legacy: unknown'}</p></div>

						<div class="actions">
							<button type="button" disabled={!$canWrite || resolvingCrashes[getCrashKey(crash)] || resolvingAllCrashes} on:click={() => openTicketModal(crash)}>
								Make Ticket
							</button>
							<button type="button" disabled={!$canWrite || resolvingCrashes[getCrashKey(crash)] || resolvingAllCrashes} on:click={() => resolveCrash(crash)}>
								{resolvingCrashes[getCrashKey(crash)] ? 'Resolving...' : 'Resolve group'}
							</button>
						</div>
					</div>
				</div>
			</Panel>
		{/each}
		</div>
	{/if}
 <CrashHealth {selectedProject} refreshToken={healthRefresh} on:loaded={e=>health=e.detail} on:filter={e=>{buildFilter=e.detail;document.getElementById('crash-queue')?.scrollIntoView({block:'start'})}}/>

</main>

<PlankaTicketModal
	open={!!ticketCrash}
	title={ticketCrash ? ticketName(ticketCrash) : ''}
	description={ticketCrash ? ticketDescription(ticketCrash) : ''}
	on:close={() => (ticketCrash = null)}
	on:created={handleTicketCreated}
/>

<style>
 .incident-overview{display:flex;flex-wrap:wrap;gap:24px;padding:10px 0 18px}.incident-overview div{display:flex;align-items:baseline;gap:9px}.incident-overview span{font-size:.75rem;color:var(--text-soft)}.incident-overview b{font:700 1.15rem var(--font-mono);color:var(--forest)}.stats.primary-stats{grid-template-columns:repeat(2,minmax(90px,max-content))}.queue-heading{scroll-margin-top:60px}

 .queue-controls{display:flex;align-items:end;flex-wrap:wrap;gap:12px;margin-bottom:16px}.queue-controls label{display:flex;flex-direction:column;gap:5px;font-size:.7rem;font-weight:700;color:var(--text-soft);min-width:0}.queue-controls select{min-width:0}.queue-controls .search-control{flex:1;min-width:190px}.search-control input{box-sizing:border-box;width:100%;padding:7px 9px;border:1px solid var(--line-strong);border-radius:4px;background:var(--surface);color:var(--text);font:inherit;font-size:.82rem}.issue-state{display:flex;gap:6px;flex-wrap:wrap;margin-bottom:8px}.legacy-badge,.build-filter{font-size:.76rem;text-align:left;overflow-wrap:anywhere}.build-filter button{font-size:.74rem;margin-left:8px}@media(max-width:550px){.queue-controls label{width:100%}.queue-controls select{width:100%}}

	main {
		color: var(--text);
		max-width: 1400px;
		margin: 0 auto;
		padding: 12px 16px 32px;
	}

	.build-cutoff-control {
		align-items: flex-start;
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.build-cutoff-control span {
		color: var(--text-soft);
		font-size: .7rem;
		font-weight: 800;
		letter-spacing: .05em;
		text-transform: uppercase;
	}

	select {
		background: var(--surface);
		border: 1px solid var(--line-strong);
		border-radius: 4px;
		color: var(--text);
		font: inherit;
		font-size: 0.82rem;
		min-width: 220px;
		padding: 6px 9px;
	}

	.save-status {
		align-items: center;
		color: var(--text-soft);
		display: flex;
		font-size: 0.82rem;
		font-weight: 700;
		gap: 6px;
		min-height: 30px;
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

	.comment-list {
		display: flex;
		flex-direction: column;
		gap: 10px;
	}



	:global(.comment.resolving) {
		opacity: 0.45;
		pointer-events: none;
	}

	.comment-body {
		align-items: stretch;
		display: grid;
		gap: 18px;
		grid-template-columns: minmax(0, 1fr) minmax(210px, auto);
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

	:global(.comment) p {
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

	.meta-list .project-tag {
		background: var(--surface);
		border-color: var(--forest);
		color: var(--charcoal);
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
  font-size: clamp(1.25rem, 1.5vw, 1.5rem);
  font-weight: 700;
  line-height: 1.25;
  margin: 0 0 8px;
  overflow-wrap: anywhere;
  text-align: left;
 }

	.stack-frame {
		align-items: baseline;
		color: var(--text-muted);
		display: grid;
		font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
		font-size: 0.78rem;
		gap: 10px;
		grid-template-columns: 24px minmax(0, 1fr);
		line-height: 1.5;
		overflow-wrap: anywhere;
		padding: 2px 0;
		text-align: left;
	}

	.stack-frame span {
		color: var(--text-soft);
		font-variant-numeric: tabular-nums;
		text-align: left;
	}

	.crash-details {
		background: var(--paper-warm);
		border: 1px solid var(--line);
		border-radius: 6px;
		margin-top: 8px;
		padding: 6px 8px;
	}

	.crash-details dl {
		display: grid;
		gap: 3px 10px;
		grid-template-columns: repeat(auto-fit, minmax(min(100%, 205px), 1fr));
		margin: 0;
	}

	.crash-details dl div {
		align-items: baseline;
		display: flex;
		gap: 5px;
		min-width: 0;
	}

	.crash-details dt {
		color: var(--text-soft);
		flex: 0 1 auto;
		font-size: 0.68rem;
		font-weight: 700;
	}

	.crash-details dd {
		color: var(--charcoal);
		font-size: 0.74rem;
		margin: 0;
		overflow-wrap: anywhere;
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

		.build-cutoff-control,
		.build-cutoff-control select {
			width: 100%;
		}







		.comment-list {
			gap: 12px;
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




	}

	@media (max-width: 480px) {
		.stats {
			grid-template-columns: 1fr;
		}




	}
</style>
