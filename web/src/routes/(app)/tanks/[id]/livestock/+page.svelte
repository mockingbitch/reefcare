<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import {
		api,
		type ApiError,
		type LivestockEntry,
		type Page,
		type Tank
	} from '#lib/api/client.js';
	import Alert from '#lib/components/Alert.svelte';
	import { errorMessage } from '#lib/errors.js';
	import {
		coralTypeLabel,
		groupByCategory,
		livestockCategoryLabel,
		livestockFormLabel,
		speciesName
	} from '#lib/livestock.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';

	const id = $derived(page.params.id ?? '');
	const locale = getLocale();

	let tank = $state<Tank | null>(null);
	let entries = $state<LivestockEntry[]>([]);
	let showGone = $state(false);
	let loading = $state(true);
	let error = $state<ApiError | null>(null);

	const canEdit = $derived(tank?.access === 'owner' && tank.status !== 'archived');
	const groups = $derived(groupByCategory(entries));

	async function load() {
		loading = true;
		error = null;
		const base = `/tanks/${encodeURIComponent(id)}`;
		const [t, l] = await Promise.all([
			api<Tank>('GET', base),
			api<Page<LivestockEntry>>('GET', `${base}/livestock${showGone ? '?include=gone' : ''}`)
		]);
		loading = false;
		if (!t.ok) return void (error = t.error);
		if (!l.ok) return void (error = l.error);
		tank = t.data;
		entries = l.data.items;
	}

	$effect(() => {
		void id;
		void showGone;
		load();
	});
</script>

<svelte:head><title>{m.ls_title()} · {tank?.name ?? ''} · {m.app_name()}</title></svelte:head>

<p><a href={resolve('/(app)/tanks/[id]', { id })}>← {tank?.name ?? m.nav_tanks()}</a></p>
<h1>{m.ls_title()}</h1>

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

	{#if canEdit}
		<div class="actions">
			<a class="button" href={resolve('/(app)/tanks/[id]/livestock/new', { id })}>{m.ls_add()}</a>
		</div>
	{/if}
	<label class="check">
		<input type="checkbox" bind:checked={showGone} />
		{m.ls_show_gone()}
	</label>

	{#if entries.length === 0}
		<p class="muted">{m.ls_empty()}</p>
	{:else}
		{#each groups as [category, list] (category)}
			<section class="card" aria-labelledby="cat-{category}">
				<h2 id="cat-{category}">
					{livestockCategoryLabel(category)}
					<span class="muted"
						>({list.filter((e) => e.present).reduce((n, e) => n + e.current_quantity, 0)})</span
					>
				</h2>
				<ul class="list">
					{#each list as e (e.id)}
						<li class="item">
							<div class="row">
								<a href={resolve('/(app)/tanks/[id]/livestock/[entryId]', { id, entryId: e.id })}
									><strong>{e.display_name}</strong></a
								>
								<span class="qty">× {e.current_quantity}</span>
								{#if !e.present}<span class="badge">{m.ls_gone_badge()}</span>{/if}
								{#if e.quarantine_status === 'in_quarantine'}
									<span class="badge">⚠ {m.ls_quarantine_in()}</span>
								{/if}
								{#if e.coral_type}<span class="badge">{coralTypeLabel(e.coral_type)}</span>{/if}
								{#if e.form}<span class="badge">{livestockFormLabel(e.form)}</span>{/if}
							</div>
							{#if e.species}
								<p class="muted">
									{speciesName(e.species, locale)}{#if e.species.scientific_name}
										· <em>{e.species.scientific_name}</em>{/if}
								</p>
							{/if}
							{#if e.location_note}<p>{m.ls_location_value({ place: e.location_note })}</p>{/if}
						</li>
					{/each}
				</ul>
			</section>
		{/each}
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
		padding: 0.6rem 0;
	}
	.item:first-child {
		border-top: 0;
	}
	.row {
		display: flex;
		flex-wrap: wrap;
		gap: 0.4rem 0.75rem;
		align-items: center;
	}
	.item p {
		margin: 0.2rem 0;
	}
	.qty {
		font-weight: 600;
	}
	.check {
		display: inline-flex;
		gap: 0.4rem;
		align-items: center;
		font-weight: normal;
		margin: 0.5rem 0 1rem;
	}
</style>
