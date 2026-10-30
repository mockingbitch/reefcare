<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { api, type ApiError, type User } from '#lib/api/client.js';
	import Alert from '#lib/components/Alert.svelte';
	import Field from '#lib/components/Field.svelte';
	import { errorMessage, fieldErrors } from '#lib/errors.js';
	import { m } from '#lib/paraglide/messages.js';
	import { session } from '#lib/session.svelte.js';
	import { safeNext } from '#lib/token.js';

	let email = $state('');
	let password = $state('');
	let busy = $state(false);
	let error = $state<ApiError | null>(null);
	const fields = $derived(fieldErrors(error));

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = null;
		const r = await api<{ user: User; csrf_token: string }>('POST', '/auth/login', {
			email,
			password
		});
		busy = false;
		if (!r.ok) {
			error = r.error;
			return;
		}
		const target = safeNext(page.url.searchParams.get('next')) ?? resolve('/(app)/tanks');
		// target is either resolve() output or a same-origin path checked by safeNext.
		// eslint-disable-next-line svelte/no-navigation-without-resolve
		await session.signIn(r.data.user, target, (url) => goto(url));
	}
</script>

<svelte:head><title>{m.login_title()} · {m.app_name()}</title></svelte:head>

<h1>{m.login_title()}</h1>

<form class="card" onsubmit={submit} novalidate>
	{#if error}<Alert kind="error">{errorMessage(error)}</Alert>{/if}
	<Field
		label={m.field_email()}
		name="email"
		type="email"
		autocomplete="email"
		required
		bind:value={email}
		error={fields.email}
	/>
	<Field
		label={m.field_password()}
		name="password"
		type="password"
		autocomplete="current-password"
		required
		bind:value={password}
		error={fields.password}
	/>
	<div class="actions">
		<button type="submit" disabled={busy}>{busy ? m.loading() : m.login_submit()}</button>
		<a href={resolve('/(auth)/forgot-password')}>{m.forgot_password_link()}</a>
	</div>
</form>

<p>{m.no_account()} <a href={resolve('/(auth)/register')}>{m.nav_register()}</a></p>
