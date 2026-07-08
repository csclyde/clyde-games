<script>	
</script>

<main>
	<h2>About Me</h2>
	<p>I travel when I can. I work when I have to. I love my wife and daughter. I sometimes find joy in building things. I do the best I'm able to do.</p>

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
		max-width: var(--content);
		margin: 0 auto;
		padding: 64px 24px 24px;
		flex-direction: column;
		align-items: center;
	}

	h2,
	p {
		text-align: left;
	}

	h2 {
		border-bottom: 1px solid var(--line);
		padding-bottom: 16px;
	}

	p {
		color: var(--text-muted);
		font-size: 1.25rem;
		font-weight: 500;
		line-height: 1.45;
		max-width: 760px;
	}

	section {
		display: grid;
		gap: 16px;
		grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
		margin: 36px auto 0;
	}

	img {
		aspect-ratio: 1;
		background: var(--surface-muted);
		border: 1px solid var(--line);
		border-radius: var(--radius);
		filter: saturate(0.78) contrast(0.96);
		height: auto;
		object-fit: cover;
		padding: 6px;
		width: 100%;
	}
</style>
