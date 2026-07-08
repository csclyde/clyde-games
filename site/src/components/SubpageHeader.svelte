<script>
	export let label = '';
</script>

<header>
	<nav>
		<a href="/">← Back</a>
		{#if label}
			<span>{label}</span>
		{/if}
	</nav>
</header>
<div class="header-spacer" aria-hidden="true"></div>

<style>
	header {
		background: rgba(255, 254, 250, 0.86);
		border-bottom: 1px solid var(--line);
		left: 0;
		position: fixed;
		right: 0;
		top: 0;
		z-index: 10;
		backdrop-filter: blur(14px);
	}

	.header-spacer {
		height: 41px;
	}

	nav {
		align-items: center;
		display: flex;
		gap: 12px;
		justify-content: space-between;
		margin: 0 auto;
		max-width: var(--content);
		padding: 12px 24px;
	}

	a,
	span {
		font-family: var(--font-mono);
		font-size: 0.78rem;
		font-weight: 700;
		letter-spacing: 0;
		line-height: 1;
		text-transform: uppercase;
	}

	a {
		color: var(--charcoal);
		text-decoration: none;
	}

	a:hover {
		color: var(--slate);
	}

	span {
		color: var(--text-soft);
		text-align: right;
	}

	@media (max-width: 640px) {
		nav {
			padding: 11px 16px;
		}

		.header-spacer {
			height: 39px;
		}
	}
</style>
