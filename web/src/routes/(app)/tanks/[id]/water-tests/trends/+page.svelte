<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { api, type ApiError, type ParameterType, type Trends } from '#lib/api/client.js';
	import Alert from '#lib/components/Alert.svelte';
	import TrendChart from '#lib/components/TrendChart.svelte';
	import { errorMessage } from '#lib/errors.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import { session } from '#lib/session.svelte.js';
	import { loadCatalog, parameterLabel } from '#lib/water-tests.js';

	const id = $derived(page.params.id ?? '');
	const locale = getLocale();
	const tz = $derived(session.user?.timezone ?? 'Asia/Ho_Chi_Minh');
	const ranges = [30, 90, 365] as const;

	let catalog = $state<ParameterType[]>([]);
	let selected = $state<string[]>(['alkalinity', 'calcium', 'magnesium', 'nitrate', 'phosphate']);
	let days = $state<number>(90);
	let trends = $state<Trends | null>(null);
	let loading = $state(true);
	let error = $state<ApiError | null>(null);

	async function load() {
		loading = true;
		error = null;
		const c = await loadCatalog();
		if (!c.ok) {
			loading = false;
			return void (error = c.error);
		}
		catalog = c.data;
		if (selected.length === 0) {
			loading = false;
			trends = null;
			return;
		}
		const from = new Date(Date.now() - days * 86_400_000).toISOString();
		const r = await api<Trends>(
			'GET',
			`/tanks/${encodeURIComponent(id)}/water-tests/trends?parameters=${encodeURIComponent(selected.join(','))}&from=${encodeURIComponent(from)}`
		);
		loading = false;
		if (!r.ok) return void (error = r.error);
		trends = r.data;
	}

	$effect(() => {
		void id;
		void days;
		void selected.length;
		load();
	});
</script>

<svelte:head><title>{m.wp_trends()} · {m.app_name()}</title></svelte:head>

<p><a href={resolve('/(app)/tanks/[id]/water-tests', { id })}>← {m.wp_title()}</a></p>
<h1>{m.wp_trends()}</h1>

<form class="filters card" onsubmit={(e) => e.preventDefault()}>
	<fieldset>
		<legend>{m.wp_range()}</legend>
		{#each ranges as d (d)}
			<label class="check"
				><input type="radio" name="range" value={d} bind:group={days} />
				{m.wp_last_days({ days: String(d) })}</label
			>
		{/each}
	</fieldset>
	<fieldset>
		<legend>{m.wp_parameters()}</legend>
		{#each catalog as p (p.code)}
			<label class="check"
				><input type="checkbox" value={p.code} bind:group={selected} />
				{parameterLabel(p.code)}</label
			>
		{/each}
	</fieldset>
</form>

{#if error}
	{#if error.code === 'not_found'}
		<p>{m.not_found_body()}</p>
	{:else}
		<Alert kind="error">
			{errorMessage(error)}
			<button type="button" class="secondary" onclick={load}>{m.retry()}</button>
		</Alert>
	{/if}
{:else if selected.length === 0}
	<p class="muted">{m.wp_pick_parameter()}</p>
{:else if loading && !trends}
	<p role="status">{m.loading()}</p>
{:else if trends}
	{#if trends.truncated}<Alert kind="warning">{m.wp_truncated()}</Alert>{/if}
	{#if trends.annotations_truncated}<Alert kind="info">{m.wp_annotations_truncated()}</Alert>{/if}
	<div aria-busy={loading}>
		{#each trends.series as s (s.parameter)}
			<TrendChart
				series={s}
				annotations={trends.annotations}
				from={trends.from}
				to={trends.to}
				{locale}
				timeZone={tz}
			/>
		{/each}
	</div>
{/if}

<style>
	.filters {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem 2rem;
	}
	fieldset {
		border: 0;
		margin: 0;
		padding: 0;
		display: flex;
		flex-wrap: wrap;
		gap: 0.25rem 1rem;
	}
	legend {
		font-weight: 600;
		margin-bottom: 0.25rem;
	}
</style>
