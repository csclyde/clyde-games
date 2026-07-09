<script>
	export let name;
	export let href = '';
	export let img;
	export let desc;
	export let tools;
	export let accent = 'var(--slate)';
	
	let target = href.includes('http') ? '_blank' : '_self';
	let rel = target === '_blank' ? 'noreferrer' : undefined;
</script>

<a class="project-card" { href } { target } {rel} style:--project-accent={accent}>
	<h3>{ name }</h3>

	{#if img != undefined}
		<img src={img} alt="A picture of { name }"/>
	{/if}

	<p>{ desc }</p>
	<small><i>Built with: { tools }</i></small>
</a>

<style>
	.project-card {
		background: var(--surface);
		border: 1px solid var(--line);
		border-left: 4px solid var(--project-accent);
		border-radius: var(--radius);
		color: inherit;
		display: flex;
		flex-direction: column;
		min-height: 100%;
		padding: 14px;
		text-decoration: none;
		transition:
			background-color 160ms ease,
			border-color 160ms ease,
			box-shadow 160ms ease,
			transform 160ms ease;
	}

	.project-card:hover,
	.project-card:focus-visible {
		background: color-mix(in srgb, var(--surface) 86%, var(--project-accent));
		border-color: color-mix(in srgb, var(--line) 70%, var(--project-accent));
		box-shadow: 0 12px 22px color-mix(in srgb, var(--project-accent) 18%, transparent);
		transform: translateY(-3px) scale(1.01);
	}

	.project-card:focus-visible {
		outline: 3px solid color-mix(in srgb, var(--project-accent) 65%, white);
		outline-offset: 3px;
	}

	h3 {
		color: var(--charcoal);
		font-size: 1.55rem;
		font-weight: 800;
		line-height: 1;
		margin: 0 0 16px;
		text-align: left;
	}

	.project-card:hover h3,
	.project-card:focus-visible h3 {
		color: var(--project-accent);
	}

	p {
		color: var(--text-muted);
		font-weight: 500;
		flex-grow: 1;
		line-height: 1.45;
		margin: 14px 0;
		text-align: left;
	}

	small {
		border-top: 1px solid var(--line);
		color: var(--project-accent);
		display: block;
		font-weight: 700;
		padding-top: 10px;
		text-align: left;
		text-transform: uppercase;
	}

	img {
		aspect-ratio: 5 / 4;
		border: 1px solid var(--line);
		border-radius: 2px;
		filter: saturate(0.92) contrast(0.98);
		height: auto;
		object-fit: cover;
		width: 100%;
	}
</style>
