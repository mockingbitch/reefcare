<script lang="ts">
	import { resolve } from '$app/paths';
	import { api, type ApiError } from '#lib/api/client.js';
	import Alert from '#lib/components/Alert.svelte';
	import Field from '#lib/components/Field.svelte';
	import { errorMessage, fieldErrors } from '#lib/errors.js';
	import { m } from '#lib/paraglide/messages.js';

	let email = $state('');
	let busy = $state(false);
	let sent = $state(false);
	let error = $state<ApiError | null>(null);
	const fields = $derived(fieldErrors(error));

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = null;
		const r = await api('POST', '/auth/password-reset/request', { email });
		busy = false;
		if (r.ok) sent = true;
		else error = r.error;
	}
</script>

<svelte:head><title>{m.forgot_title()} · {m.app_name()}</title></svelte:head>

<h1>{m.forgot_title()}</h1>

{#if sent}
	<Alert kind="success">{m.forgot_sent()}</Alert>
	<p><a href={resolve('/(auth)/login')}>{m.nav_login()}</a></p>
{:else}
	<form class="card" onsubmit={submit} novalidate>
		<p>{m.forgot_intro()}</p>
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
		<div class="actions">
			<button type="submit" disabled={busy}>{busy ? m.loading() : m.forgot_submit()}</button>
		</div>
	</form>
{/if}
