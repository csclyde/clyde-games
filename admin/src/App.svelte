<script>
	import router from 'page';
	import Admin from './pages/Admin.svelte';

	let tab = 'admin';
	let userID = '';

	router('/', () => router.redirect('/metrics'));
	router('/admin', () => tab = 'admin');
	router('/metrics', () => tab = 'admin');
	router('/feedback', () => tab = 'feedback');
	router('/crashes', () => tab = 'crashes');
	router('/savegames', () => tab = 'savegames');
	router('/save-games', () => router.redirect('/savegames'));
	router('/tools', () => tab = 'tools');
	router('/users', () => router.redirect('/admin'));
	router('/user/:id', context => {
		userID = context.params.id;
		tab = 'user';
	});
	router.start();
</script>

<Admin {tab} {userID} />
