<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { api, type ApiError, type Equipment, type Page, type Tank } from '#lib/api/client.js';
	import Alert from '#lib/components/Alert.svelte';
	import { categoryLabel, warrantyExpired } from '#lib/equipment.js';
	import { errorMessage } from '#lib/errors.js';
	import { formatDate, formatDateTime, today } from '#lib/format.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import { session } from '#lib/session.svelte.js';

	const id = $derived(page.params.id ?? '');
	const locale = getLocale();
	const tz = $derived(session.user?.timezone ?? 'Asia/Ho_Chi_Minh');

	let tank = $state<Tank | null>(null);
	let devices = $state<Equipment[]>([]);
	let showRemoved = $state(false);
	let loading = $state(true);
	let error = $state<ApiError | null>(null);

	const canEdit = $derived(tank?.access === 'owner' && tank.status !== 'archived');

	async function load() {
		loading = true;
		error = null;
		const base = `/tanks/${encodeURIComponent(id)}`;
		const [t, d] = await Promise.all([
			api<Tank>('GET', base),
			api<Page<Equipment>>('GET', `${base}/equipment${showRemoved ? '?include=removed' : ''}`)
		]);
		loading = false;
		if (!t.ok) return void (error = t.error);
		if (!d.ok) return void (error = d.error);
		tank = t.data;
		devices = d.data.items;
	}

	$effect(() => {
		void id;
		void showRemoved;
		load();
	});
</script>

<svelte:head><title>{m.eq_title()} · {tank?.name ?? ''} · {m.app_name()}</title></svelte:head>

<p><a href={resolve('/(app)/tanks/[id]', { id })}>← {tank?.name ?? m.nav_tanks()}</a></p>
<h1>{m.eq_title()}</h1>

{#if loading && !tank}
	<p role="status">{m.loading()}</p>
{:else if error}
	{#if error.code === 'not_found'}
		<p>{m.not_found_body()}</p>
	{:else}
		<Alert kind="error">
			{errorMessage(error)}
			<button type="button" class="secondary" onclick={load}>{m.retry()}</button>
		</Alert>
	{/if}
{:else if tank}
	{#if tank.access === 'viewer'}<Alert kind="info">{m.viewer_notice()}</Alert>{/if}
	{#if tank.status === 'archived'}<Alert kind="info">{m.archived_notice()}</Alert>{/if}

	<div class="actions">
		{#if canEdit}
			<a class="button" href={resolve('/(app)/tanks/[id]/equipment/new', { id })}>{m.eq_add()}</a>
		{/if}
		<a class="button secondary" href={resolve('/(app)/tanks/[id]/maintenance', { id })}
			>{m.tank_section_maintenance()}</a
		>
	</div>

	<label class="check">
		<input type="checkbox" bind:checked={showRemoved} />
		{m.eq_show_removed()}
	</label>

	{#if devices.length === 0}
		<p class="muted">{m.eq_empty()}</p>
	{:else}
		<ul class="list">
			{#each devices as d (d.id)}
				<li class="card">
					<div class="row">
						<a
							href={resolve('/(app)/tanks/[id]/equipment/[equipmentId]', { id, equipmentId: d.id })}
							><strong>{d.name}</strong></a
						>
						<span class="badge">{categoryLabel(d.category)}</span>
						{#if d.removed_at}<span class="badge">{m.eq_removed_badge()}</span>{/if}
					</div>
					{#if d.manufacturer || d.model}
						<p>{[d.manufacturer, d.model].filter(Boolean).join(' · ')}</p>
					{/if}
					<p class="muted">
						{m.eq_installed_on({ when: formatDateTime(d.installed_at, locale, tz) })}
						{#if d.removed_at}
							· {m.eq_removed_on({ when: formatDateTime(d.removed_at, locale, tz) })}{/if}
					</p>
					{#if d.warranty_until && !d.removed_at}
						<p>
							{#if warrantyExpired(d.warranty_until, today(tz))}
								<span aria-hidden="true">⚠</span>
								{m.eq_warranty_expired({ date: formatDate(d.warranty_until, locale) })}
							{:else}
								{m.eq_warranty_until({ date: formatDate(d.warranty_until, locale) })}
							{/if}
						</p>
					{/if}
					{#if d.configuration}
						<p class="muted">
							{m.eq_config_version({ n: String(d.configuration.version_no) })}
						</p>
					{/if}
				</li>
			{/each}
		</ul>
	{/if}
{/if}

<style>
	.list {
		list-style: none;
		margin: 0;
		padding: 0;
		display: grid;
		gap: 0.75rem;
	}
	.row {
		display: flex;
		flex-wrap: wrap;
		gap: 0.4rem 0.75rem;
		align-items: center;
	}
	.list p {
		margin: 0.25rem 0;
	}
	.check {
		display: inline-flex;
		gap: 0.4rem;
		align-items: center;
		font-weight: normal;
		margin: 0.5rem 0 1rem;
	}
</style>
