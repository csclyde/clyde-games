<script>
	export let name;
	export let href = '';
	export let img;
	export let desc;
	export let tools;
	export let accent = 'var(--slate)';
	
	let target = href.includes('http') ? '_blank' : '_self';
</script>

<div style:--project-accent={accent}>
	<h3><a { href } { target }>{ name }</a></h3>

	{#if img != undefined}
		<a { href } { target }><img src={img} alt="A picture of { name }"/></a>
	{/if}

	<p>{ desc }</p>
	<small><i>Built with: { tools }</i></small>
</div>

<style>
	div {
		background: var(--surface);
		border: 1px solid var(--line);
		border-left: 4px solid var(--project-accent);
		border-radius: var(--radius);
		display: flex;
		flex-direction: column;
		min-height: 100%;
		padding: 14px;
	}

	h3 {
		font-size: 1.55rem;
		font-weight: 800;
		line-height: 1;
		margin: 0 0 16px;
		text-align: left;
	}

	h3 a {
		color: var(--charcoal);
		text-decoration: none;
	}

	h3 a:hover {
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
