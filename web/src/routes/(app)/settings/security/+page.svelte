<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { api, type ApiError } from '#lib/api/client.js';
	import Alert from '#lib/components/Alert.svelte';
	import ConfirmDialog from '#lib/components/ConfirmDialog.svelte';
	import Field from '#lib/components/Field.svelte';
	import { errorMessage, fieldErrors } from '#lib/errors.js';
	import { formatDateTime } from '#lib/format.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import { session } from '#lib/session.svelte.js';

	// Change password
	let current = $state('');
	let next = $state('');
	let pwBusy = $state(false);
	let pwError = $state<ApiError | null>(null);
	let pwDone = $state(false);
	const pwFields = $derived(fieldErrors(pwError));

	async function changePassword(e: SubmitEvent) {
		e.preventDefault();
		pwBusy = true;
		pwError = null;
		pwDone = false;
		const r = await api('POST', '/me/password', { current_password: current, new_password: next });
		pwBusy = false;
		if (!r.ok) {
			pwError = r.error;
			return;
		}
		// The API rotated the session and its CSRF cookie; nothing else to do.
		current = next = '';
		pwDone = true;
	}

	// Sign out everywhere
	let allError = $state<ApiError | null>(null);
	async function logoutAll() {
		const r = await api('POST', '/auth/logout-all');
		if (!r.ok) {
			allError = r.error;
			return;
		}
		session.clear();
		await goto(resolve('/(auth)/login'));
	}

	// Delete account
	let confirmDelete = $state(false);
	let delPassword = $state('');
	let delBusy = $state(false);
	let delError = $state<ApiError | null>(null);
	let eraseAfter = $state<string | null>(null);
	const delFields = $derived(fieldErrors(delError));

	async function deleteAccount() {
		delBusy = true;
		delError = null;
		const r = await api<{ status: string; erase_after: string }>('DELETE', '/me', {
			password: delPassword
		});
		delBusy = false;
		if (!r.ok) {
			delError = r.error;
			return;
		}
		confirmDelete = false;
		eraseAfter = r.data.erase_after;
		session.clear();
	}
</script>

<svelte:head><title>{m.security_title()} · {m.app_name()}</title></svelte:head>

<h1>{m.security_title()}</h1>

{#if eraseAfter}
	<Alert kind="info">{m.deleted_notice({ date: formatDateTime(eraseAfter, getLocale()) })}</Alert>
	<p><a href={resolve('/')}>{m.go_home()}</a></p>
{:else}
	<section aria-labelledby="pw">
		<h2 id="pw">{m.change_password_title()}</h2>
		<form class="card" onsubmit={changePassword} novalidate>
			{#if pwError}<Alert kind="error">{errorMessage(pwError)}</Alert>{/if}
			{#if pwDone}<Alert kind="success">{m.password_changed()}</Alert>{/if}
			<Field
				label={m.field_current_password()}
				name="current_password"
				type="password"
				autocomplete="current-password"
				required
				bind:value={current}
				error={pwFields.current_password}
			/>
			<Field
				label={m.field_new_password()}
				name="new_password"
				type="password"
				autocomplete="new-password"
				minlength={10}
				maxlength={128}
				required
				hint={m.password_hint()}
				bind:value={next}
				error={pwFields.new_password}
			/>
			<div class="actions">
				<button type="submit" disabled={pwBusy}
					>{pwBusy ? m.saving() : m.change_password_submit()}</button
				>
			</div>
		</form>
	</section>

	<section aria-labelledby="sessions">
		<h2 id="sessions">{m.sessions_title()}</h2>
		<div class="card">
			{#if allError}<Alert kind="error">{errorMessage(allError)}</Alert>{/if}
			<p>{m.logout_all_body()}</p>
			<button type="button" class="secondary" onclick={logoutAll}>{m.logout_all_submit()}</button>
		</div>
	</section>

	<section aria-labelledby="delete">
		<h2 id="delete">{m.delete_title()}</h2>
		<div class="card">
			<p>{m.delete_body()}</p>
			<button type="button" class="danger" onclick={() => (confirmDelete = true)}
				>{m.delete_submit()}</button
			>
		</div>
	</section>

	<ConfirmDialog
		bind:open={confirmDelete}
		title={m.delete_confirm_title()}
		confirmLabel={m.delete_submit()}
		danger
		busy={delBusy}
		onconfirm={deleteAccount}
	>
		<p>{m.delete_body()}</p>
		{#if delError && !delFields.password}<Alert kind="error">{errorMessage(delError)}</Alert>{/if}
		<Field
			label={m.delete_password_label()}
			name="password"
			type="password"
			autocomplete="current-password"
			required
			bind:value={delPassword}
			error={delFields.password}
		/>
	</ConfirmDialog>
{/if}
