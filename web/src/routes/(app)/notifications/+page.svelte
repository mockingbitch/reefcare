<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { api, type ApiError, type Notification, type Page } from '#lib/api/client.js';
	import Alert from '#lib/components/Alert.svelte';
	import { errorMessage } from '#lib/errors.js';
	import { formatDateTime } from '#lib/format.js';
	import { notificationSection, notificationText } from '#lib/notifications.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import { session } from '#lib/session.svelte.js';
	import { unread } from '#lib/unread.svelte.js';

	const locale = getLocale();
	const tz = $derived(session.user?.timezone ?? 'Asia/Ho_Chi_Minh');

	let items = $state<Notification[]>([]);
	let cursor = $state<string | null>(null);
	let onlyUnread = $state(false);
	let loading = $state(true);
	let error = $state<ApiError | null>(null);
	let busy = $state(false);

	async function load() {
		loading = true;
		error = null;
		const r = await api<Page<Notification>>(
			'GET',
			`/notifications?limit=30${onlyUnread ? '&unread=true' : ''}`
		);
		loading = false;
		if (!r.ok) return void (error = r.error);
		items = r.data.items;
		cursor = r.data.next_cursor;
		unread.refresh();
	}

	$effect(() => {
		void onlyUnread;
		load();
	});

	async function more() {
		if (!cursor) return;
		const r = await api<Page<Notification>>(
			'GET',
			`/notifications?limit=30${onlyUnread ? '&unread=true' : ''}&cursor=${encodeURIComponent(cursor)}`
		);
		if (!r.ok) return void (error = r.error);
		items = [...items, ...r.data.items];
		cursor = r.data.next_cursor;
	}

	async function open(n: Notification) {
		if (!n.read_at) {
			await api('POST', `/notifications/${n.id}/read`);
			n.read_at = new Date().toISOString();
			unread.refresh();
		}
		const id = n.tank_id;
		if (!id) return;
		switch (notificationSection(n)) {
			case 'dosing':
				return goto(resolve('/(app)/tanks/[id]/dosing', { id }));
			case 'water-changes':
				return goto(resolve('/(app)/tanks/[id]/water-changes', { id }));
			case 'maintenance':
				return goto(resolve('/(app)/tanks/[id]/maintenance', { id }));
		}
	}

	async function markRead(n: Notification) {
		const r = await api('POST', `/notifications/${n.id}/read`);
		if (!r.ok) return void (error = r.error);
		n.read_at = new Date().toISOString();
		unread.refresh();
	}

	async function readAll() {
		busy = true;
		const r = await api('POST', '/notifications/read-all');
		busy = false;
		if (!r.ok) return void (error = r.error);
		await load();
	}
</script>

<svelte:head><title>{m.nt_title()} · {m.app_name()}</title></svelte:head>

<h1>{m.nt_title()}</h1>
<p class="muted">{m.nt_intro()}</p>

<div class="actions">
	<button type="button" class="secondary" disabled={busy || unread.count === 0} onclick={readAll}
		>{m.nt_read_all()}</button
	>
	<a class="button secondary" href={resolve('/(app)/settings/profile')}>{m.nt_settings()}</a>
</div>
<label class="check">
	<input type="checkbox" bind:checked={onlyUnread} />
	{m.nt_only_unread()}
</label>

{#if loading && items.length === 0}
	<p role="status">{m.loading()}</p>
{:else if error}
	<Alert kind="error">
		{errorMessage(error)}
		<button type="button" class="secondary" onclick={load}>{m.retry()}</button>
	</Alert>
{:else if items.length === 0}
	<p class="muted">{onlyUnread ? m.nt_empty_unread() : m.nt_empty()}</p>
{:else}
	<ul class="list">
		{#each items as n (n.id)}
			<li class="item" class:unread={!n.read_at}>
				<div class="row">
					{#if !n.read_at}<span class="badge">{m.nt_unread_badge()}</span>{/if}
					<time class="muted" datetime={n.occurred_at}
						>{formatDateTime(n.occurred_at, locale, tz)}</time
					>
				</div>
				<p>{notificationText(n, locale, tz)}</p>
				<div class="actions">
					{#if notificationSection(n)}
						<button type="button" onclick={() => open(n)}>{m.nt_open()}</button>
					{/if}
					{#if !n.read_at}
						<button type="button" class="secondary" onclick={() => markRead(n)}
							>{m.nt_mark_read()}</button
						>
					{/if}
				</div>
			</li>
		{/each}
	</ul>
	{#if cursor}
		<button type="button" class="secondary" onclick={more}>{m.load_more()}</button>
	{/if}
{/if}

<style>
	.list {
		list-style: none;
		margin: 0;
		padding: 0;
	}
	.item {
		border-top: 1px solid var(--border, #dde);
		padding: 0.75rem 0 0.75rem 0.75rem;
		border-left: 3px solid transparent;
	}
	.item.unread {
		border-left-color: currentColor;
	}
	.item p {
		margin: 0.25rem 0;
	}
	.row {
		display: flex;
		flex-wrap: wrap;
		gap: 0.4rem 0.75rem;
		align-items: center;
	}
	.check {
		display: inline-flex;
		gap: 0.4rem;
		align-items: center;
		font-weight: normal;
		margin: 0.5rem 0 1rem;
	}
</style>
