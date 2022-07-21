<script>	
</script>

<main>
	<h2>About Me</h2>
	<p>I travel when I can. I work when I have to. I sometimes find joy in building things. I do the best I'm able to do.</p>

	<section>
		<img src="img/kc4.jpg" alt="Good lighting">
		<img src="img/kc1.jpg" alt="In Japan">
		<img src="img/kc6.jpg" alt="Into the hole">
		<img src="img/kc2.jpg" alt="After a party">
		<img src="img/kc3.jpg" alt="Lord of the Van">
		<img src="img/kc5.jpg" alt="Mt. Rainier">
	</section>
</main>

<style>
	main {
		flex-direction: column;
		align-items: center;
	}

	section {
		display: flex;
		flex-direction: row;
		flex-wrap: wrap;
		gap: 16px;
		margin-left: auto;
		margin-right: auto;
		justify-content: center;
	}
	img {
		width: 300px;
		height: 300px;
		object-fit: cover;
		border-radius: 12px;
	}
</style>