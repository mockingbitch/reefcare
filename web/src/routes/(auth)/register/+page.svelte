<script lang="ts">
	import { resolve } from '$app/paths';
	import { api, type ApiError } from '#lib/api/client.js';
	import Alert from '#lib/components/Alert.svelte';
	import Field from '#lib/components/Field.svelte';
	import { errorMessage, fieldErrors } from '#lib/errors.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';

	let email = $state('');
	let password = $state('');
	let displayName = $state('');
	let busy = $state(false);
	let error = $state<ApiError | null>(null);
	let sentTo = $state<string | null>(null);
	const fields = $derived(fieldErrors(error));

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = null;
		const r = await api('POST', '/auth/register', {
			email,
			password,
			display_name: displayName,
			locale: getLocale(),
			timezone: Intl.DateTimeFormat().resolvedOptions().timeZone
		});
		busy = false;
		if (!r.ok) {
			error = r.error;
			return;
		}
		sentTo = email.trim();
	}
</script>

<svelte:head><title>{m.register_title()} · {m.app_name()}</title></svelte:head>

{#if sentTo}
	<h1>{m.register_sent_title()}</h1>
	<Alert kind="success">{m.register_sent_body({ email: sentTo })}</Alert>
	<p><a href={resolve('/(auth)/login')}>{m.nav_login()}</a></p>
{:else}
	<h1>{m.register_title()}</h1>
	<form class="card" onsubmit={submit} novalidate>
		{#if error}<Alert kind="error">{errorMessage(error)}</Alert>{/if}
		<Field
			label={m.field_display_name()}
			name="display_name"
			autocomplete="nickname"
			maxlength={60}
			required
			bind:value={displayName}
			error={fields.display_name}
		/>
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
			autocomplete="new-password"
			minlength={10}
			maxlength={128}
			required
			hint={m.password_hint()}
			bind:value={password}
			error={fields.password}
		/>
		{#if fields.timezone}<Alert kind="error">{fields.timezone}</Alert>{/if}
		<div class="actions">
			<button type="submit" disabled={busy}>{busy ? m.loading() : m.register_submit()}</button>
		</div>
	</form>
	<p>{m.have_account()} <a href={resolve('/(auth)/login')}>{m.nav_login()}</a></p>
{/if}
