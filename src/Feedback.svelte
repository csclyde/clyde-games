<script>

	async function getFeedback() {
		const res = await fetch(`https://clyde.games/api/feedback`);
		const feedback = await res.json();

		if (res.ok) {
			return feedback;
		} else {
			throw new Error(feedback);
		}
	}

	let promise = getFeedback();
</script>

<main>
	<h1>Player Feedback</h1>
	<hr/>

	{#await promise}
		<p>loading...</p>
	{:then feedback}
		<p>{JSON.stringify(feedback)}</p>
	{:catch error}
		<p style="color: red">{error.message}</p>
	{/await}
</main>

<style>

</style>