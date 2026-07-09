<script>
	import SubpageHeader from '../components/SubpageHeader.svelte';

	async function analyzeWords() {
		submittedText = analysisText;

		var body = {
			text: analysisText,
		}

		analysisText = '';

		const res = await fetch(`https://api.clyde.games/words/analyze`, {
			method: 'POST',
			body: JSON.stringify(body),
			headers: {
				'Content-Type': 'application/json'
			},
		});

		const words = await res.json();

		if (res.ok) {
			return words;
		} else {
			throw new Error(words);
		}
	}

	async function getWordStats() {
		const res = await fetch(`https://api.clyde.games/words/stats`, {
			method: 'POST',
		});

		const stats = await res.json();

		if (res.ok) {
			return stats;
		} else {
			throw new Error(stats);
		}
	}

	let analysisText = '';
	let submittedText = '';
	let analyzePromise = null;

	let statsPromise = getWordStats();

	function pct(n) {
		if (!Number.isFinite(n)) {
			return 0;
		}

		return (Math.floor(n * 1000) / 10)
	}

	let origins = [
		{ label: 'Unknown', key: 'Unknown', color: '#1f2937', soft: 'rgba(31, 41, 55, 0.12)' },
		{ label: 'Germanic', key: 'Germanic', color: '#047857', soft: 'rgba(4, 120, 87, 0.14)' },
		{ label: 'French', key: 'French', color: '#2563eb', soft: 'rgba(37, 99, 235, 0.14)' },
		{ label: 'Latin', key: 'Latin', color: '#c2410c', soft: 'rgba(194, 65, 12, 0.14)' },
		{ label: 'Greek', key: 'Greek', color: '#7c3aed', soft: 'rgba(124, 58, 237, 0.14)' },
		{ label: 'Other', key: 'Other', color: '#0f766e', soft: 'rgba(15, 118, 110, 0.14)' }
	];

	function statFor(stats, origin) {
		return pct(stats[origin.key] / stats.Total);
	}

	function wordLink(word) {
		return `https://www.etymonline.com/word/${encodeURIComponent(word.Text)}`;
	}

	function analyzedTokens(text, words) {
		let wordIndex = 0;

		return text.split(/(\w+)/g).filter(part => part !== '').map(part => {
			if (/^\w+$/.test(part)) {
				let word = words[wordIndex];
				wordIndex += 1;

				return {
					text: part,
					word,
					origin: word ? origins[word.Origin] || origins[0] : origins[0],
					isWord: true
				};
			}

			return {
				text: part,
				isWord: false
			};
		});
	}
</script>

<SubpageHeader label="Analyze Word Origins" />

<main>
	<header>
		<p class="eyebrow">Word Analysis</p>
		<h2>Analyze Word Origins</h2>
	</header>
	<hr/>

	<div class="composer">
		<textarea bind:value={analysisText} placeholder="Paste a paragraph, poem, or passage to analyze..."></textarea>
		<button disabled={!analysisText.trim()} on:click={() => analyzePromise = analyzeWords()}>Analyze</button>
	</div>

	{#await statsPromise}
		<p class="site-stats">loading...</p>
	{:then stats}
	<section class="site-stats" aria-label="All word origin statistics">
		<small>Total: {stats.Total}</small>
		<small>Unknown: {pct(stats.Unknown / stats.Total)}%</small>
		<small>Germanic: {pct(stats.Germanic / stats.Total)}%</small>
		<small>French: {pct(stats.French / stats.Total)}%</small>
		<small>Latin: {pct(stats.Latin / stats.Total)}%</small>
		<small>Greek: {pct(stats.Greek / stats.Total)}%</small>
		<small>Other: {pct(stats.Other / stats.Total)}%</small>
	</section>
	{:catch error}
		<p class="site-stats" style="color: var(--rust)">{error.message}</p>
	{/await}

	{#if analyzePromise != null}
	{#await analyzePromise}
	<p>loading...</p>
	{:then analysis}
	<div class="analysis-panel">
		<div class="analysis-header">
			<div>
				<p class="eyebrow">Analyzed Passage</p>
				<h3>{analysis.Statistics.Total} words mapped</h3>
			</div>
			<div class="origin-bars" aria-label="Analyzed text origin breakdown">
				{#each origins as origin}
					<span
						title="{origin.label}: {statFor(analysis.Statistics, origin)}%"
						style="background:{origin.color}; flex-grow:{analysis.Statistics[origin.key] || 0}"
					></span>
				{/each}
			</div>
		</div>

		<section class="legend" aria-label="Origin legend">
			{#each origins as origin}
				<div class="legend-item" style="--origin:{origin.color}; --origin-soft:{origin.soft}">
					<span class="swatch"></span>
					<span>{origin.label}</span>
					<strong>{statFor(analysis.Statistics, origin)}%</strong>
				</div>
			{/each}
		</section>

		<article class="passage" aria-label="Analyzed text">
			{#each analyzedTokens(submittedText, analysis.Words) as token}
				{#if token.isWord && token.word}
					<a
						class="analyzed-word"
						target="_blank"
						rel="noreferrer"
						href={wordLink(token.word)}
						style="--origin:{token.origin.color}; --origin-soft:{token.origin.soft}"
						title="{token.origin.label}: {token.word.Text}"
					>{token.text}</a>
				{:else}
					<span class="punctuation">{token.text}</span>
				{/if}
			{/each}
		</article>
	</div>
	{:catch error}
	<p style="color: var(--rust)">{error.message}</p>
	{/await}
	{/if}

</main>

<style>
	main {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 18px;
		margin: 0 auto;
		max-width: var(--content);
		padding: 42px 24px 24px;
	}

	header {
		text-align: left;
		width: 100%;
	}

	h2,
	h3,
	.eyebrow {
		text-align: left;
	}

	h3 {
		margin: 4px 0 0;
	}

	.eyebrow {
		color: var(--text-soft);
		font-family: var(--font-mono);
		font-size: 0.75rem;
		font-weight: 700;
		letter-spacing: 0.12em;
		margin: 0 0 8px;
		text-transform: uppercase;
	}

	hr {
		border: 0;
		border-top: 1px solid var(--line);
		width: 100%;
	}

	.composer {
		align-items: center;
		display: flex;
		flex-direction: column;
		gap: 14px;
		width: 100%;
	}

	textarea {
		background:
			linear-gradient(180deg, rgba(255, 254, 250, 0.98), rgba(255, 254, 250, 0.9)),
			var(--surface);
		border: 1px solid var(--line-strong);
		border-radius: 8px;
		box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.72), 0 16px 34px rgba(39, 43, 41, 0.06);
		color: var(--text);
		font-family: var(--font-sans);
		font-size: 1.08rem;
		line-height: 1.65;
		width: min(900px, 100%);
		min-height: 260px;
		padding: 18px 20px;
		resize: vertical;
		text-align: left;
	}

	textarea:focus {
		border-color: var(--charcoal);
		box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.72), 0 18px 42px rgba(39, 43, 41, 0.1);
		outline: none;
	}

	textarea::placeholder {
		color: var(--text-soft);
	}

	section {
		display: flex;
		gap: 16px;
		flex-wrap: wrap;
	}

	button {
		background: var(--surface);
		border: 1px solid var(--line-strong);
		border-radius: var(--radius);
		color: var(--text);
		cursor: pointer;
		font: inherit;
		font-weight: 700;
		padding: 8px 12px;
	}

	button:disabled {
		cursor: not-allowed;
		opacity: 0.45;
	}

	button:hover {
		background: var(--paper-warm);
		border-color: var(--charcoal);
	}

	button:disabled:hover {
		background: var(--surface);
		border-color: var(--line-strong);
	}

	.site-stats {
		justify-content: center;
		margin-top: -2px;
		max-width: 900px;
		text-align: center;
		width: min(900px, 100%);
	}

	.site-stats small {
		text-align: center;
	}

	.analysis-panel {
		background:
			linear-gradient(180deg, rgba(255, 254, 250, 0.96), rgba(255, 254, 250, 0.84)),
			var(--surface);
		border: 1px solid var(--line);
		border-radius: 8px;
		box-shadow: 0 18px 48px rgba(39, 43, 41, 0.08);
		display: flex;
		flex-direction: column;
		gap: 20px;
		margin-top: 18px;
		padding: 24px;
		width: min(960px, 100%);
	}

	.analysis-header {
		align-items: end;
		display: grid;
		gap: 18px;
		grid-template-columns: minmax(0, 1fr) minmax(220px, 38%);
	}

	.origin-bars {
		background: var(--surface-muted);
		border: 1px solid var(--line);
		border-radius: 999px;
		display: flex;
		height: 14px;
		overflow: hidden;
	}

	.origin-bars span {
		min-width: 6px;
	}

	.legend {
		display: grid;
		gap: 8px;
		grid-template-columns: repeat(3, minmax(0, 1fr));
		width: 100%;
	}

	.legend-item {
		align-items: center;
		background: var(--origin-soft);
		border: 1px solid color-mix(in srgb, var(--origin) 26%, transparent);
		border-radius: var(--radius);
		color: var(--text);
		display: grid;
		font-family: var(--font-mono);
		font-size: 0.78rem;
		gap: 8px;
		grid-template-columns: auto 1fr auto;
		padding: 8px 10px;
	}

	.swatch {
		background: var(--origin);
		border-radius: 50%;
		height: 10px;
		width: 10px;
	}

	.passage {
		background:
			linear-gradient(90deg, rgba(142, 149, 143, 0.18) 1px, transparent 1px),
			var(--paper);
		background-size: 32px 100%;
		border-left: 4px solid var(--charcoal);
		color: var(--text);
		font-size: clamp(1.1rem, 1.7vw, 1.45rem);
		line-height: 1.95;
		padding: 22px 24px;
		text-align: left;
		white-space: pre-wrap;
	}

	.analyzed-word {
		background: linear-gradient(180deg, transparent 52%, var(--origin-soft) 52%);
		border-bottom: 2px solid color-mix(in srgb, var(--origin) 68%, transparent);
		color: var(--origin);
		font-weight: 700;
		text-decoration: none;
		transition: background 120ms ease, color 120ms ease;
	}

	.analyzed-word:hover {
		background: var(--origin-soft);
		color: var(--origin);
	}

	.punctuation {
		color: var(--text-muted);
	}

	@media (max-width: 720px) {
		main {
			padding-top: 40px;
		}

		.analysis-header {
			align-items: start;
			grid-template-columns: 1fr;
		}

		.legend {
			grid-template-columns: 1fr;
		}

		.analysis-panel {
			padding: 18px;
		}

		.passage {
			font-size: 1.08rem;
			padding: 18px;
		}
	}
</style>
