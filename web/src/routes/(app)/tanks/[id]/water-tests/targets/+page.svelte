<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import {
		api,
		type ApiError,
		type Page,
		type ParameterType,
		type Tank,
		type Target
	} from '#lib/api/client.js';
	import Alert from '#lib/components/Alert.svelte';
	import Field from '#lib/components/Field.svelte';
	import { errorMessage, fieldErrors } from '#lib/errors.js';
	import { formatDateTime } from '#lib/format.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import { session } from '#lib/session.svelte.js';
	import {
		loadCatalog,
		parameterLabel,
		targetRowErrors,
		targetRows,
		targetsToReplace,
		targetText,
		unitLabel,
		type TargetRow
	} from '#lib/water-tests.js';

	const id = $derived(page.params.id ?? '');
	const locale = getLocale();
	const tz = $derived(session.user?.timezone ?? 'Asia/Ho_Chi_Minh');
	const base = $derived(`/tanks/${encodeURIComponent(id)}/parameter-targets`);

	let tank = $state<Tank | null>(null);
	let catalog = $state<ParameterType[]>([]);
	let current = $state<Target[]>([]);
	let rows = $state<TargetRow[]>([]);
	let loading = $state(true);
	let loadError = $state<ApiError | null>(null);
	let busy = $state(false);
	let error = $state<ApiError | null>(null);
	let rowIndex = $state<number[]>([]);
	let savedAt = $state<string | null>(null);

	const canEdit = $derived(tank?.access === 'owner' && tank.status !== 'archived');
	const fields = $derived(targetRowErrors(fieldErrors(error), rowIndex));

	async function load() {
		loading = true;
		loadError = null;
		const [t, c, g] = await Promise.all([
			api<Tank>('GET', `/tanks/${encodeURIComponent(id)}`),
			loadCatalog(),
			api<Page<Target>>('GET', base)
		]);
		loading = false;
		for (const r of [t, c, g]) if (!r.ok) return void (loadError = r.error);
		if (!t.ok || !c.ok || !g.ok) return;
		tank = t.data;
		catalog = c.data;
		current = g.data.items;
		rows = targetRows(c.data, g.data.items);
	}

	$effect(() => {
		void id;
		load();
	});

	async function save(e: SubmitEvent) {
		e.preventDefault();
		const req = targetsToReplace(rows);
		rowIndex = req.rowIndex;
		busy = true;
		error = null;
		savedAt = null;
		const r = await api<Page<Target>>('PUT', base, req.body);
		busy = false;
		if (!r.ok) return void (error = r.error);
		current = r.data.items;
		rows = targetRows(catalog, r.data.items);
		savedAt = new Date().toISOString();
	}

	const unitOf = (code: string) => catalog.find((c) => c.code === code)?.canonical_unit ?? '';
	const defaultDays = (code: string) =>
		String(Math.round((catalog.find((c) => c.code === code)?.freshness_hours ?? 0) / 24));
</script>

<svelte:head><title>{m.wp_targets()} · {tank?.name ?? ''} · {m.app_name()}</title></svelte:head>

<p><a href={resolve('/(app)/tanks/[id]/water-tests', { id })}>← {m.wp_title()}</a></p>
<h1>{m.wp_targets()}</h1>

{#if loading && !tank}
	<p role="status">{m.loading()}</p>
{:else if loadError}
	{#if loadError.code === 'not_found'}
		<p>{m.not_found_body()}</p>
	{:else}
		<Alert kind="error">
			{errorMessage(loadError)}
			<button type="button" class="secondary" onclick={load}>{m.retry()}</button>
		</Alert>
	{/if}
{:else if tank && !canEdit}
	{#if tank.access === 'viewer'}<Alert kind="info">{m.viewer_notice()}</Alert>{/if}
	{#if tank.status === 'archived'}<Alert kind="info">{m.archived_notice()}</Alert>{/if}
	{#if current.length === 0}
		<p class="muted">{m.wp_targets_empty()}</p>
	{:else}
		<dl class="facts">
			{#each current as t (t.parameter)}
				<dt>{parameterLabel(t.parameter)}</dt>
				<dd>{targetText(t, unitOf(t.parameter), locale) || '—'}</dd>
			{/each}
		</dl>
	{/if}
{:else if tank}
	<p>{m.wp_targets_intro()}</p>
	<form onsubmit={save} novalidate>
		{#if savedAt}<Alert kind="success">{m.wp_targets_saved()}</Alert>{/if}
		{#if error}<Alert kind="error">{errorMessage(error)}</Alert>{/if}
		{#each rows as row, i (row.parameter)}
			{@const unit = unitLabel(unitOf(row.parameter))}
			{@const since = current.find((c) => c.parameter === row.parameter)?.valid_from}
			<fieldset>
				<legend>{parameterLabel(row.parameter)}{unit ? ` · ${unit}` : ''}</legend>
				{#if since}
					<p class="hint">{m.wp_target_since({ when: formatDateTime(since, locale, tz) })}</p>
				{/if}
				<div class="cols">
					<Field
						label={m.wp_min()}
						name="min_{row.parameter}"
						inputmode="decimal"
						bind:value={row.min}
						error={fields[`${i}.min`]}
					/>
					<Field
						label={m.wp_ideal()}
						name="ideal_{row.parameter}"
						inputmode="decimal"
						bind:value={row.ideal}
						error={fields[`${i}.ideal`]}
					/>
					<Field
						label={m.wp_max()}
						name="max_{row.parameter}"
						inputmode="decimal"
						bind:value={row.max}
						error={fields[`${i}.max`]}
					/>
					<Field
						label={m.wp_freshness_days()}
						name="fresh_{row.parameter}"
						inputmode="decimal"
						bind:value={row.freshnessDays}
						placeholder={defaultDays(row.parameter)}
						error={fields[`${i}.freshnessDays`]}
					/>
				</div>
			</fieldset>
		{/each}
		<div class="actions">
			<button type="submit" disabled={busy}>{m.save()}</button>
		</div>
	</form>
{/if}

<style>
	fieldset {
		border: 1px solid var(--border);
		border-radius: var(--radius);
		padding: 0.5rem 1rem;
		margin: 0 0 0.75rem;
	}
	legend {
		font-weight: 600;
	}
	.cols {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(7rem, 1fr));
		gap: 0 0.75rem;
	}
</style>
