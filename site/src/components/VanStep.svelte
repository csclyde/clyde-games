<script>
	export let step;
</script>

<section>
	<img src={step.img} alt="Step in the conversion process"/>
	<p>{ step.desc }</p>
</section>

<style>
	section {
		background: var(--surface);
		border-radius: var(--radius);
		border: 1px solid var(--line);
		border-left: 4px solid var(--ochre);
		padding: 14px;
		display: flex;
		flex-direction: column;
		align-items: center;
	}

	p {
		color: var(--text-muted);
		font-weight: 500;
		flex-grow: 1;
		line-height: 1.45;
		text-align: left;
	}

	img {
		aspect-ratio: 1;
		border: 1px solid var(--line);
		border-radius: 2px;
		filter: saturate(0.78) contrast(0.96);
		height: auto;
		object-fit: cover;
		width: 100%;
	}
</style>
