<script lang="ts">
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { api, type ApiError } from '#lib/api/client.js';
	import Alert from '#lib/components/Alert.svelte';
	import { errorMessage } from '#lib/errors.js';
	import { m } from '#lib/paraglide/messages.js';
	import { session } from '#lib/session.svelte.js';
	import { takeFragmentToken } from '#lib/token.js';

	let status = $state<'working' | 'done' | 'missing' | 'failed'>('working');
	let error = $state<ApiError | null>(null);

	onMount(async () => {
		const token = takeFragmentToken();
		if (!token) {
			status = 'missing';
			return;
		}
		const r = await api('POST', '/auth/verify-email', { token });
		if (r.ok) {
			status = 'done';
			if (session.user) await session.load(fetch, true);
		} else {
			error = r.error;
			status = 'failed';
		}
	});
</script>

<svelte:head>
	<title>{m.verify_title()} · {m.app_name()}</title>
	<meta name="referrer" content="no-referrer" />
</svelte:head>

<h1>{m.verify_title()}</h1>

{#if status === 'working'}
	<p role="status">{m.verify_working()}</p>
{:else if status === 'done'}
	<Alert kind="success">{m.verify_success()}</Alert>
	<p>
		<a class="button" href={session.user ? resolve('/(app)/tanks') : resolve('/(auth)/login')}
			>{m.continue()}</a
		>
	</p>
{:else if status === 'missing'}
	<Alert kind="error">{m.verify_missing_token()}</Alert>
{:else if error}
	<Alert kind="error">{errorMessage(error)}</Alert>
	<p><a href={resolve('/(auth)/login')}>{m.nav_login()}</a></p>
{/if}
