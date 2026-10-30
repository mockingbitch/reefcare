<script lang="ts">
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { api, type ApiError } from '#lib/api/client.js';
	import Alert from '#lib/components/Alert.svelte';
	import Field from '#lib/components/Field.svelte';
	import { errorMessage, fieldErrors } from '#lib/errors.js';
	import { m } from '#lib/paraglide/messages.js';
	import { session } from '#lib/session.svelte.js';
	import { takeFragmentToken } from '#lib/token.js';

	let token = $state<string | null>(null);
	let checked = $state(false);
	let password = $state('');
	let busy = $state(false);
	let done = $state(false);
	let error = $state<ApiError | null>(null);
	const fields = $derived(fieldErrors(error));

	onMount(() => {
		token = takeFragmentToken();
		checked = true;
	});

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = null;
		const r = await api('POST', '/auth/password-reset/confirm', { token, new_password: password });
		busy = false;
		if (r.ok) {
			done = true;
			session.clear(); // the reset signed out every session
		} else {
			error = r.error;
		}
	}
</script>

<svelte:head>
	<title>{m.reset_title()} · {m.app_name()}</title>
	<meta name="referrer" content="no-referrer" />
</svelte:head>

<h1>{m.reset_title()}</h1>

{#if done}
	<Alert kind="success">{m.reset_success()}</Alert>
	<p><a class="button" href={resolve('/(auth)/login')}>{m.nav_login()}</a></p>
{:else if checked && !token}
	<Alert kind="error">{m.verify_missing_token()}</Alert>
	<p><a href={resolve('/(auth)/forgot-password')}>{m.forgot_title()}</a></p>
{:else if checked}
	<form class="card" onsubmit={submit} novalidate>
		{#if error}<Alert kind="error">{errorMessage(error)}</Alert>{/if}
		<Field
			label={m.field_new_password()}
			name="new_password"
			type="password"
			autocomplete="new-password"
			minlength={10}
			maxlength={128}
			required
			hint={m.password_hint()}
			bind:value={password}
			error={fields.new_password}
		/>
		<div class="actions">
			<button type="submit" disabled={busy}>{busy ? m.loading() : m.reset_submit()}</button>
		</div>
	</form>
{/if}
