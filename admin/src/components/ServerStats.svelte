<script>
 import Panel from './Panel.svelte';
	import { onDestroy } from 'svelte';

	const refreshInterval = 15000;
	let stats = null;
	let error = '';
	let refreshing = false;
	let apiResponseMs = 0;
	let timer;

	async function loadStats() {
		refreshing = true;
		const started = performance.now();
		try {
			const response = await fetch('/api/system/stats', { cache: 'no-store' });
			const body = await response.json();
			if (!response.ok) throw new Error(body.error || 'Server statistics are unavailable');
			stats = body;
			apiResponseMs = Math.round(performance.now() - started);
			error = '';
		} catch (loadError) {
			error = loadError.message || 'Server statistics are unavailable';
		} finally {
			refreshing = false;
		}
	}

	function severity(percent) {
		if (percent >= 90) return 'critical';
		if (percent >= 75) return 'warning';
		return 'healthy';
	}

	function bytes(value) {
		const units = ['B', 'KB', 'MB', 'GB', 'TB'];
		let size = value || 0;
		let unit = 0;
		while (size >= 1024 && unit < units.length - 1) {
			size /= 1024;
			unit += 1;
		}
		return `${size.toFixed(unit > 2 ? 1 : 0)} ${units[unit]}`;
	}

	function uptime(seconds) {
		const days = Math.floor(seconds / 86400);
		const hours = Math.floor((seconds % 86400) / 3600);
		return days ? `${days}d ${hours}h` : `${hours}h ${Math.floor((seconds % 3600) / 60)}m`;
	}

	loadStats();
	timer = setInterval(loadStats, refreshInterval);

	onDestroy(() => clearInterval(timer));
</script>

<Panel class="server-health" title="Server Health" subtitle={stats ? `Up ${uptime(stats.uptime)} · updated ${new Date(stats.collectedAt).toLocaleTimeString()}` : "Reading server statistics…"}>
 <button slot="actions" type="button" on:click={loadStats} disabled={refreshing}>{refreshing ? 'Refreshing…' : 'Refresh'}</button>

	{#if !stats && refreshing}
        <div class="stat-grid" aria-busy="true" aria-label="Reading server statistics">
            {#each ['CPU', 'Memory', 'Disk', 'API', 'Database'] as name, index}
                <div class="stat" class:health-stat={index > 2}>
                    <div class="stat-label"><span>{name}</span><strong>--</strong></div>
                    {#if index < 3}<div class="meter"></div>{/if}
                    <p>Waiting for reading...</p>
                </div>
            {/each}
        </div>
	{:else if !stats}
		<p class="message error">{error}</p>
	{:else}
		<div class="stat-grid">
			<div class="stat" data-severity={severity(stats.cpuPercent)}>
				<div class="stat-label"><span>CPU</span><strong>{stats.cpuPercent.toFixed(1)}%</strong></div>
				<div class="meter"><span style={`width: ${stats.cpuPercent}%`}></span></div>
				<p>{stats.cpuCores} logical core{stats.cpuCores === 1 ? '' : 's'}</p>
			</div>
			<div class="stat" data-severity={severity(stats.memory.percent)}>
				<div class="stat-label"><span>Memory</span><strong>{stats.memory.percent.toFixed(1)}%</strong></div>
				<div class="meter"><span style={`width: ${stats.memory.percent}%`}></span></div>
				<p>{bytes(stats.memory.used)} of {bytes(stats.memory.total)}</p>
			</div>
			<div class="stat" data-severity={severity(stats.disk.percent)}>
				<div class="stat-label"><span>Disk</span><strong>{stats.disk.percent.toFixed(1)}%</strong></div>
				<div class="meter"><span style={`width: ${stats.disk.percent}%`}></span></div>
				<p>{bytes(stats.disk.used)} of {bytes(stats.disk.total)}</p>
			</div>
			<div class="stat health-stat" data-severity="healthy">
				<div class="stat-label"><span>API</span><strong>Healthy</strong></div>
				<p>Responded in {apiResponseMs} ms</p>
			</div>
			<div class="stat health-stat" data-severity={stats.database.healthy ? 'healthy' : 'critical'}>
				<div class="stat-label"><span>Database</span><strong>{stats.database.healthy ? 'Healthy' : 'Unavailable'}</strong></div>
				<p>{bytes(stats.database.size)} across {stats.database.connections} databases · {stats.database.responseTimeMs} ms</p>
			</div>
		</div>
		{#if error}<p class="stale-warning">Refresh failed: {error}. Showing the last reading.</p>{/if}
	{/if}
</Panel>

<style>




	.stat p { color: var(--text-muted); font-size: .75rem; margin: 3px 0 0; }
	button { min-width: 106px; flex-shrink: 0; background: var(--paper-warm); border: 1px solid var(--line); border-radius: 4px; color: var(--text); cursor: pointer; font: inherit; font-size: .75rem; font-weight: 700; padding: 6px 9px; }
	button:disabled { cursor: wait; opacity: .6; }
	.stat-grid { display: grid; gap: 12px; grid-template-columns: repeat(6, 1fr); }
	.stat { grid-column: span 2; }
	.health-stat { grid-column: span 3; }
	.stat { --status: #66814a; background: var(--paper-warm); border-left: 3px solid var(--status); border-radius: 3px; padding: 10px; }
	.stat[data-severity='warning'] { --status: #bd7a22; }
	.stat[data-severity='critical'] { --status: #b7473d; }
	.stat-label { align-items: baseline; display: flex; justify-content: space-between; }
	.stat-label span { font-size: .72rem; font-weight: 800; letter-spacing: .05em; text-transform: uppercase; }
	.stat-label strong { font-size: 1.1rem; }
	.meter { background: var(--line); border-radius: 999px; height: 5px; margin-top: 7px; overflow: hidden; }
	.meter span { background: var(--status); display: block; height: 100%; transition: width .3s ease; }
	.message, .stale-warning { color: var(--text-muted); font-size: .82rem; margin: 6px 0; }
	.error, .stale-warning { color: #a13c34; }
	@media (max-width: 850px) { .stat-grid { grid-template-columns: 1fr; } .stat, .health-stat { grid-column: auto; } }
	@media (max-width: 650px) {    }
</style>
