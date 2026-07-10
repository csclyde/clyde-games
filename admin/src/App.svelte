<script>
	import router from 'page';
	import Admin from './pages/Admin.svelte';

	let tab = 'admin';

	router('/', () => router.redirect('/metrics'));
	router('/admin', () => tab = 'admin');
	router('/metrics', () => tab = 'admin');
	router('/feedback', () => tab = 'feedback');
	router('/crashes', () => tab = 'crashes');
	router('/savegames', () => tab = 'savegames');
	router('/save-games', () => router.redirect('/savegames'));
	router.start();
</script>

<Admin {tab} />
