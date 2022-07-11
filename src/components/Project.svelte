<script>
	export let name;
	export let href = '';
	export let img;
	export let desc;
	export let tools;
	
	let target = href.includes('http') ? '_blank' : '_self';

</script>

<div>
	<h1><a { href } { target }>{ name }</a></h1>

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
	}

	p {
		font-weight: 500;
		max-width: 330px;
		margin-left: auto;
		margin-right: auto;
	}

	img {
		border-radius: 12px;
		width: 315px;
		height: 250px;
		object-fit: cover;
	}
</style>