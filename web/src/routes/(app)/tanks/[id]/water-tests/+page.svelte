<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import {
		api,
		type ApiError,
		type LatestReading,
		type Page,
		type ParameterType,
		type Reading,
		type Tank,
		type WaterTest
	} from '#lib/api/client.js';
	import Alert from '#lib/components/Alert.svelte';
	import ConfirmDialog from '#lib/components/ConfirmDialog.svelte';
	import Field from '#lib/components/Field.svelte';
	import SelectField from '#lib/components/SelectField.svelte';
	import { errorMessage, fieldErrors } from '#lib/errors.js';
	import { formatDateTime } from '#lib/format.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import { session } from '#lib/session.svelte.js';
	import { decimalInput } from '#lib/tank-form.js';
	import {
		freshnessText,
		loadCatalog,
		methodLabel,
		parameterLabel,
		targetStatusText,
		targetText,
		unitLabel,
		valueText,
		warningLabel
	} from '#lib/water-tests.js';

	const id = $derived(page.params.id ?? '');
	const locale = getLocale();
	const tz = $derived(session.user?.timezone ?? 'Asia/Ho_Chi_Minh');

	let tank = $state<Tank | null>(null);
	let catalog = $state<ParameterType[]>([]);
	let latest = $state<LatestReading[]>([]);
	let tests = $state<WaterTest[]>([]);
	let cursor = $state<string | null>(null);
	let filter = $state('');
	let loading = $state(true);
	let error = $state<ApiError | null>(null);

	const canEdit = $derived(tank?.access === 'owner' && tank.status !== 'archived');
	const base = $derived(`/tanks/${encodeURIComponent(id)}`);
	const measured = $derived(latest.filter((l) => l.reading));

	function testsPath(after?: string) {
		const q = [`limit=20`];
		if (filter) q.push(`parameter=${encodeURIComponent(filter)}`);
		if (after) q.push(`cursor=${encodeURIComponent(after)}`);
		return `${base}/water-tests?${q.join('&')}`;
	}

	async function load() {
		loading = true;
		error = null;
		const [t, c, l, h] = await Promise.all([
			api<Tank>('GET', base),
			loadCatalog(),
			api<Page<LatestReading>>('GET', `${base}/water-tests/latest`),
			api<Page<WaterTest>>('GET', testsPath())
		]);
		loading = false;
		for (const r of [t, c, l, h]) if (!r.ok) return void (error = r.error);
		if (!t.ok || !c.ok || !l.ok || !h.ok) return;
		tank = t.data;
		catalog = c.data;
		latest = l.data.items;
		tests = h.data.items;
		cursor = h.data.next_cursor;
	}

	$effect(() => {
		void id;
		void filter;
		load();
	});

	let moreError = $state<ApiError | null>(null);
	async function more() {
		if (!cursor) return;
		const r = await api<Page<WaterTest>>('GET', testsPath(cursor));
		if (!r.ok) return void (moreError = r.error);
		moreError = null;
		tests = [...tests, ...r.data.items];
		cursor = r.data.next_cursor;
	}

	/** Valid readings first in catalog order; corrections listed under the reading they replaced. */
	function validReadings(t: WaterTest) {
		const order = (p: string) => catalog.findIndex((c) => c.code === p);
		return t.readings
			.filter((r) => r.status === 'valid')
			.sort((a, b) => order(a.parameter) - order(b.parameter));
	}
	function history(t: WaterTest, r: Reading): Reading[] {
		const out: Reading[] = [];
		let cur: Reading | undefined = r;
		while (cur?.supersedes_id) {
			const prev = t.readings.find((x) => x.id === cur!.supersedes_id);
			if (!prev) break;
			out.push(prev);
			cur = prev;
		}
		return out;
	}
	const reasonOf = (t: WaterTest, superseded: Reading) =>
		t.readings.find((x) => x.supersedes_id === superseded.id)?.correction_reason ?? '';

	// --- Correction -----------------------------------------------------------------------------
	let fixing = $state<Reading | null>(null);
	let fixOpen = $state(false);
	let fixValue = $state<string | number>('');
	let fixUnit = $state('');
	let fixReason = $state('');
	let fixBusy = $state(false);
	let fixError = $state<ApiError | null>(null);
	const fixFields = $derived(fieldErrors(fixError));
	const fixUnits = $derived(catalog.find((c) => c.code === fixing?.parameter)?.units ?? []);

	function startFix(r: Reading) {
		fixing = r;
		fixValue = r.input_value;
		fixUnit = r.input_unit;
		fixReason = '';
		fixError = null;
		fixOpen = true;
	}

	async function saveFix() {
		if (!fixing) return;
		fixBusy = true;
		const r = await api<Reading>('POST', `/readings/${encodeURIComponent(fixing.id)}/corrections`, {
			value: decimalInput(String(fixValue)),
			unit: fixUnit,
			reason: fixReason.trim()
		});
		fixBusy = false;
		if (!r.ok) return void (fixError = r.error);
		fixOpen = false;
		await load();
	}
</script>

<svelte:head><title>{m.wp_title()} · {tank?.name ?? ''} · {m.app_name()}</title></svelte:head>

<p><a href={resolve('/(app)/tanks/[id]', { id })}>← {tank?.name ?? m.nav_tanks()}</a></p>
<h1>{m.wp_title()}</h1>

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
			<a class="button" href={resolve('/(app)/tanks/[id]/water-tests/new', { id })}
				>{m.wp_record()}</a
			>
		{/if}
		<a class="button secondary" href={resolve('/(app)/tanks/[id]/water-tests/trends', { id })}
			>{m.wp_trends()}</a
		>
		<a class="button secondary" href={resolve('/(app)/tanks/[id]/water-tests/targets', { id })}
			>{m.wp_targets()}</a
		>
	</div>

	<section class="card" aria-labelledby="latest">
		<h2 id="latest">{m.wp_latest()}</h2>
		<p class="muted">{m.wp_provisional_note()}</p>
		{#if measured.length === 0}
			<p>{m.wp_empty()}</p>
		{/if}
		<ul class="grid">
			{#each latest as l (l.parameter)}
				<li class="tile" class:stale={l.stale}>
					<span class="label">{parameterLabel(l.parameter)}</span>
					{#if l.reading}
						<strong class="value"
							>{valueText(l.reading.canonical_value, l.canonical_unit, locale)}</strong
						>
						{#if l.reading.input_unit !== l.canonical_unit}
							<span class="muted small"
								>{m.wp_entered_as({
									value: valueText(l.reading.input_value, l.reading.input_unit, locale)
								})}</span
							>
						{/if}
						{#if l.reading.estimate}<span class="small">≈ {m.wp_estimate()}</span>{/if}
						{#if targetStatusText(l.reading.target_status)}
							<span class="status {l.reading.target_status}"
								>{targetStatusText(l.reading.target_status)}</span
							>
						{/if}
					{:else}
						<strong class="value muted">—</strong>
					{/if}
					<span class="small" class:warn={l.stale && l.reading}
						>{l.stale && l.reading ? '⚠ ' : ''}{freshnessText(l, locale)}</span
					>
					{#if l.target && targetText(l.target, l.canonical_unit, locale)}
						<span class="muted small"
							>{m.wp_target_value({ range: targetText(l.target, l.canonical_unit, locale) })}</span
						>
					{/if}
				</li>
			{/each}
		</ul>
	</section>

	<section class="card" aria-labelledby="history">
		<h2 id="history">{m.wp_history()}</h2>
		<SelectField
			label={m.wp_filter_parameter()}
			name="parameter"
			bind:value={filter}
			options={[
				{ value: '', label: m.wp_all_parameters() },
				...catalog.map((c) => ({ value: c.code, label: parameterLabel(c.code) }))
			]}
		/>
		{#if tests.length === 0}
			<p class="muted">{m.wp_history_empty()}</p>
		{:else}
			<ul class="tests">
				{#each tests as t (t.id)}
					<li class="test">
						<h3>
							<time datetime={t.measured_at}>{formatDateTime(t.measured_at, locale, tz)}</time>
						</h3>
						{#if t.notes}<p class="notes">{t.notes}</p>{/if}
						<table>
							<caption class="visually-hidden"
								>{m.wp_test_caption({ when: formatDateTime(t.measured_at, locale, tz) })}</caption
							>
							<thead>
								<tr>
									<th scope="col">{m.wp_parameter()}</th>
									<th scope="col">{m.wp_value()}</th>
									<th scope="col">{m.wp_details()}</th>
									{#if canEdit}<th scope="col"
											><span class="visually-hidden">{m.wp_actions()}</span></th
										>{/if}
								</tr>
							</thead>
							<tbody>
								{#each validReadings(t) as r (r.id)}
									<tr>
										<th scope="row">{parameterLabel(r.parameter)}</th>
										<td>
											{valueText(r.input_value, r.input_unit, locale)}
											{#if r.input_unit !== r.canonical_unit}
												<span class="muted"
													>= {valueText(r.canonical_value, r.canonical_unit, locale)}</span
												>
											{/if}
										</td>
										<td
											><div class="details">
												{#if targetStatusText(r.target_status)}
													<span class="status {r.target_status}"
														>{targetStatusText(r.target_status)}</span
													>
												{/if}
												{#if r.estimate}<span>≈ {m.wp_estimate()}</span>{/if}
												{#if r.method}<span>{methodLabel(r.method)}</span>{/if}
												{#if r.test_kit}<span>{r.test_kit}</span>{/if}
												{#each r.warnings as w (w)}
													<span class="warn">⚠ {warningLabel(w)}</span>
												{/each}
												{#each history(t, r) as old (old.id)}
													<span class="muted old"
														>{m.wp_corrected_from({
															value: valueText(old.input_value, old.input_unit, locale),
															reason: reasonOf(t, old)
														})}</span
													>
												{/each}
											</div></td
										>
										{#if canEdit}
											<td>
												<button
													type="button"
													class="secondary small-btn"
													onclick={() => startFix(r)}
													aria-label={m.wp_correct_label({
														parameter: parameterLabel(r.parameter)
													})}>{m.wp_correct()}</button
												>
											</td>
										{/if}
									</tr>
								{/each}
							</tbody>
						</table>
					</li>
				{/each}
			</ul>
			{#if moreError}<Alert kind="error">{errorMessage(moreError)}</Alert>{/if}
			{#if cursor}
				<button type="button" class="secondary" onclick={more}>{m.load_more()}</button>
			{/if}
		{/if}
	</section>

	<ConfirmDialog
		bind:open={fixOpen}
		title={m.wp_correct_title({ parameter: parameterLabel(fixing?.parameter ?? '') })}
		confirmLabel={m.wp_correct_save()}
		busy={fixBusy}
		onconfirm={saveFix}
	>
		<p class="muted">{m.wp_correct_body()}</p>
		{#if fixError && !fixError.details?.length}<Alert kind="error">{errorMessage(fixError)}</Alert
			>{/if}
		<Field
			label={m.wp_value()}
			name="value"
			inputmode="decimal"
			bind:value={fixValue}
			error={fixFields.value}
			required
		/>
		<SelectField
			label={m.wp_unit()}
			name="unit"
			bind:value={fixUnit}
			options={fixUnits.map((u) => ({ value: u.code, label: unitLabel(u.code) || u.code }))}
			error={fixFields.unit}
		/>
		<Field
			label={m.wp_reason()}
			name="reason"
			bind:value={fixReason}
			error={fixFields.reason}
			required
			maxlength={500}
		/>
	</ConfirmDialog>
{/if}

<style>
	.grid {
		list-style: none;
		margin: 0;
		padding: 0;
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(10.5rem, 1fr));
		gap: 0.75rem;
	}
	.tile {
		display: flex;
		flex-direction: column;
		gap: 0.15rem;
		border: 1px solid var(--border);
		border-radius: var(--radius);
		padding: 0.6rem 0.75rem;
	}
	.tile.stale {
		border-style: dashed;
	}
	.label {
		font-weight: 600;
	}
	.value {
		font-size: 1.3rem;
	}
	.small {
		font-size: 0.85rem;
	}
	.warn {
		color: var(--warn-fg);
	}
	.status {
		font-size: 0.85rem;
		font-weight: 600;
	}
	.status.below,
	.status.above {
		color: var(--warn-fg);
	}
	.status.within {
		color: var(--ok-fg);
	}
	.tests {
		list-style: none;
		margin: 0;
		padding: 0;
	}
	.test {
		border-top: 1px solid var(--border);
		padding: 0.75rem 0;
	}
	.test:first-child {
		border-top: 0;
	}
	h3 {
		font-size: 1rem;
		margin: 0 0 0.25rem;
	}
	.notes {
		white-space: pre-line;
		margin: 0 0 0.4rem;
	}
	table {
		border-collapse: collapse;
		width: 100%;
	}
	th,
	td {
		text-align: left;
		vertical-align: top;
		padding: 0.3rem 0.6rem 0.3rem 0;
		border-bottom: 1px solid var(--border);
	}
	thead th {
		font-size: 0.85rem;
		color: var(--muted);
		font-weight: 600;
	}
	.details {
		display: flex;
		flex-wrap: wrap;
		gap: 0.2rem 0.75rem;
		font-size: 0.9rem;
	}
	.old {
		flex-basis: 100%;
	}
	.small-btn {
		padding: 0.2rem 0.6rem;
		font-size: 0.85rem;
	}
	@media (max-width: 34rem) {
		thead {
			display: none;
		}
		tr {
			display: grid;
			grid-template-columns: 1fr auto;
			border-bottom: 1px solid var(--border);
			padding: 0.3rem 0;
		}
		th,
		td {
			border: 0;
			padding: 0.1rem 0;
		}
		td:has(> .details) {
			grid-column: 1 / -1;
		}
	}
</style>
