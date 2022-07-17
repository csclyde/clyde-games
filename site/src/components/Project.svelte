<script>
	export let name;
	export let href = '';
	export let img;
	export let desc;
	export let tools;
	
	let target = href.includes('http') ? '_blank' : '_self';
</script>

<div>
	<h3><a { href } { target }>{ name }</a></h3>

	{#if img != undefined}
		<a { href } { target }><img src={img} alt="A picture of { name }"/></a>
	{/if}

	<p>{ desc }</p>
	<small><i>Built with: { tools }</i></small>
</div>

<style>
	div {
		border-radius: 6px;
		border: 2px solid black;
		margin: 16px;
		padding: 16px;
		/* min-width: 300px; */
		display: flex;
		flex-direction: column;
	}

	p {
		font-weight: 500;
		max-width: 330px;
		flex-grow: 1;
	}

	img {
		border-radius: 12px;
		width: 315px;
		height: 250px;
		object-fit: cover;
	}
</style>