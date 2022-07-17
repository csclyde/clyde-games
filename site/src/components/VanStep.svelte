<script>
	export let step;
</script>

<section>
	<img src={step.img} alt="Step in the conversion process"/>
	<p>{ step.desc }</p>
</section>

<style>
	section {
		border-radius: 6px;
		border: 2px solid black;
		margin: 8px;
		padding: 16px;
		display: flex;
		flex-direction: column;
		align-items: center;
	}

	p {
		font-weight: 500;
		flex-grow: 1;
		max-width: 300px;
	}

	img {
		border-radius: 12px;
		width: 300px;
		height: 300px;
		object-fit: cover;
	}
</style>