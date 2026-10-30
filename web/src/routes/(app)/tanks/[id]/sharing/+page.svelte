<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { api, type ApiError, type Page, type Share } from '#lib/api/client.js';
	import Alert from '#lib/components/Alert.svelte';
	import Field from '#lib/components/Field.svelte';
	import { errorMessage, fieldErrors } from '#lib/errors.js';
	import { formatDateTime } from '#lib/format.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import { session } from '#lib/session.svelte.js';

	const id = $derived(page.params.id ?? '');
	let shares = $state<Share[]>([]);
	let loading = $state(true);
	let loadError = $state<ApiError | null>(null);
	let by = $state<'email' | 'username'>('email');
	let who = $state('');
	let busy = $state(false);
	let error = $state<ApiError | null>(null);
	let notice = $state('');
	const fields = $derived(fieldErrors(error));
	const locale = getLocale();

	async function load() {
		loading = true;
		loadError = null;
		const r = await api<Page<Share>>('GET', `/tanks/${encodeURIComponent(id)}/shares`);
		loading = false;
		if (r.ok) shares = r.data.items;
		else loadError = r.error;
	}

	$effect(() => {
		void id;
		load();
	});

	async function grant(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = null;
		notice = '';
		const r = await api<Share>('POST', `/tanks/${encodeURIComponent(id)}/shares`, {
			[by]: who.trim()
		});
		busy = false;
		if (!r.ok) {
			error = r.error;
			return;
		}
		notice = m.share_added({ name: r.data.display_name });
		who = '';
		await load();
	}

	async function revoke(s: Share) {
		notice = '';
		error = null;
		const r = await api('DELETE', `/tanks/${encodeURIComponent(id)}/shares/${s.user_id}`);
		if (!r.ok) {
			error = r.error;
			return;
		}
		notice = m.share_revoked();
		await load();
	}
</script>

<svelte:head><title>{m.sharing_title()} · {m.app_name()}</title></svelte:head>

<p><a href={resolve('/(app)/tanks/[id]', { id })}>← {m.back()}</a></p>
<h1>{m.sharing_title()}</h1>
<p>{m.sharing_intro()}</p>

{#if loadError}
	<Alert kind="error">
		{errorMessage(loadError)}
		<button type="button" class="secondary" onclick={load}>{m.retry()}</button>
	</Alert>
{:else}
	{#if notice}<Alert kind="success">{notice}</Alert>{/if}

	<form class="card" onsubmit={grant} novalidate>
		{#if error}<Alert kind="error">{errorMessage(error)}</Alert>{/if}
		<fieldset>
			<legend>{m.share_by()}</legend>
			<label class="radio"
				><input type="radio" name="by" value="email" bind:group={by} /> {m.share_by_email()}</label
			>
			<label class="radio"
				><input type="radio" name="by" value="username" bind:group={by} />
				{m.share_by_username()}</label
			>
		</fieldset>
		{#if by === 'email'}
			<Field
				label={m.field_email()}
				name="email"
				type="email"
				autocomplete="off"
				required
				bind:value={who}
				error={fields.email}
			/>
		{:else}
			<Field
				label={m.field_username()}
				name="username"
				autocomplete="off"
				required
				bind:value={who}
				error={fields.username}
			/>
		{/if}
		<div class="actions">
			<button type="submit" disabled={busy || !session.user?.email_verified}
				>{busy ? m.loading() : m.share_submit()}</button
			>
		</div>
		{#if session.user && !session.user.email_verified}
			<p class="hint">{m.err_email_not_verified()}</p>
		{/if}
	</form>

	<h2>{m.shares_list()}</h2>
	{#if loading}
		<p role="status">{m.loading()}</p>
	{:else if shares.length === 0}
		<p class="muted">{m.shares_empty()}</p>
	{:else}
		<ul class="plain">
			{#each shares as s (s.user_id)}
				<li class="card share">
					<div>
						<strong>{s.display_name}</strong>
						{#if s.username}<span class="muted">@{s.username}</span>{/if}
						<div class="muted">
							{m.shared_since({
								date: formatDateTime(s.created_at, locale, session.user?.timezone)
							})}
						</div>
					</div>
					<button
						type="button"
						class="secondary"
						aria-label={m.share_revoke_label({ name: s.display_name })}
						onclick={() => revoke(s)}>{m.share_revoke()}</button
					>
				</li>
			{/each}
		</ul>
	{/if}
{/if}

<style>
	.radio {
		display: inline-flex;
		align-items: center;
		gap: 0.4rem;
		min-height: 44px;
		margin-right: 1.5rem;
	}
	.share {
		display: flex;
		justify-content: space-between;
		align-items: center;
		gap: 1rem;
		flex-wrap: wrap;
	}
</style>
