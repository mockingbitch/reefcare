<script lang="ts">
	import { resolve } from '$app/paths';
	import { api, type ApiError, type Page, type Tank } from '#lib/api/client.js';
	import Alert from '#lib/components/Alert.svelte';
	import { errorMessage } from '#lib/errors.js';
	import { formatDecimal } from '#lib/format.js';
	import { statusLabel, tankTypeLabel } from '#lib/labels.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';

	let includeShared = $state(true);
	let includeArchived = $state(false);
	let tanks = $state<Tank[]>([]);
	let next = $state<string | null>(null);
	let loading = $state(true);
	let error = $state<ApiError | null>(null);

	async function fetchPage(cursor: string | null) {
		const include = [includeShared && 'shared', includeArchived && 'archived']
			.filter(Boolean)
			.join(',');
		const params: Record<string, string> = {};
		if (include) params.include = include;
		if (cursor) params.cursor = cursor;
		const q = new URLSearchParams(params).toString();
		loading = true;
		error = null;
		const r = await api<Page<Tank>>('GET', `/tanks?${q}`);
		loading = false;
		if (!r.ok) {
			error = r.error;
			return;
		}
		tanks = cursor ? [...tanks, ...r.data.items] : r.data.items;
		next = r.data.next_cursor;
	}

	$effect(() => {
		// Re-read when a filter changes.
		void includeShared;
		void includeArchived;
		fetchPage(null);
	});

	const locale = getLocale();
</script>

<svelte:head><title>{m.tanks_title()} · {m.app_name()}</title></svelte:head>

<div class="head">
	<h1>{m.tanks_title()}</h1>
	<a class="button" href={resolve('/(app)/tanks/new')}>{m.tanks_new()}</a>
</div>

<fieldset class="filters">
	<legend class="visually-hidden">{m.tanks_title()}</legend>
	<label><input type="checkbox" bind:checked={includeShared} /> {m.show_shared()}</label>
	<label><input type="checkbox" bind:checked={includeArchived} /> {m.show_archived()}</label>
</fieldset>

{#if error}
	<Alert kind="error">
		{errorMessage(error)}
		<button type="button" class="secondary" onclick={() => fetchPage(null)}>{m.retry()}</button>
	</Alert>
{:else if loading && tanks.length === 0}
	<p role="status">{m.loading()}</p>
{:else if tanks.length === 0}
	<div class="card empty">
		<p>{m.tanks_empty()}</p>
		<a class="button" href={resolve('/(app)/tanks/new')}>{m.tanks_empty_cta()}</a>
	</div>
{:else}
	<ul class="plain tanks" aria-busy={loading}>
		{#each tanks as t (t.id)}
			<li class="card">
				<h2><a href={resolve('/(app)/tanks/[id]', { id: t.id })}>{t.name}</a></h2>
				<p class="meta">
					<span class="badge">{tankTypeLabel(t.tank_type)}</span>
					<span class="badge">{statusLabel(t.status)}</span>
					<span class="badge">{t.access === 'owner' ? m.access_owner() : m.access_viewer()}</span>
				</p>
				<p class="muted">
					{m.volume_estimate({ value: formatDecimal(t.system_volume_l, locale) })}
				</p>
			</li>
		{/each}
	</ul>
	{#if next}
		<button type="button" class="secondary" disabled={loading} onclick={() => fetchPage(next)}>
			{loading ? m.loading() : m.load_more()}
		</button>
	{/if}
{/if}

<style>
	.head {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: space-between;
		gap: 0.5rem;
	}
	.filters {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem 1.5rem;
		padding-bottom: 0.75rem;
	}
	.filters label {
		display: flex;
		align-items: center;
		gap: 0.4rem;
		min-height: 44px;
	}
	.tanks h2 {
		margin: 0 0 0.4rem;
	}
	.meta {
		display: flex;
		flex-wrap: wrap;
		gap: 0.4rem;
		margin: 0 0 0.4rem;
	}
	.empty {
		text-align: center;
	}
</style>
