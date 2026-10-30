<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { api, type ApiError, type ParameterType, type WaterTest } from '#lib/api/client.js';
	import Alert from '#lib/components/Alert.svelte';
	import Field from '#lib/components/Field.svelte';
	import SelectField from '#lib/components/SelectField.svelte';
	import { errorMessage, fieldErrors } from '#lib/errors.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import { session } from '#lib/session.svelte.js';
	import { fromLocalInput, toLocalInput } from '#lib/water-changes.js';
	import {
		emptyRows,
		loadCatalog,
		methodLabel,
		methods,
		parameterLabel,
		rowErrors,
		testToCreate,
		unitLabel,
		valueText,
		warningLabel,
		type ReadingRow,
		type UnitPrefs
	} from '#lib/water-tests.js';

	const id = $derived(page.params.id ?? '');
	const tz = session.user?.timezone ?? 'Asia/Ho_Chi_Minh';
	const locale = getLocale();

	let catalog = $state<ParameterType[]>([]);
	let rows = $state<ReadingRow[]>([]);
	// The input has minute precision; left untouched, "now" is sent as null so the server uses
	// its own clock (a truncated minute could predate a target saved seconds ago).
	const initialMeasuredAt = toLocalInput(new Date().toISOString(), tz);
	let measuredAt = $state(initialMeasuredAt);
	let notes = $state('');
	let loadError = $state<ApiError | null>(null);
	let busy = $state(false);
	let error = $state<ApiError | null>(null);
	let rowIndex = $state<number[]>([]);
	let saved = $state<WaterTest | null>(null);
	// One key per form: a retried submit records the test once.
	const idempotencyKey = crypto.randomUUID();

	const fields = $derived(rowErrors(fieldErrors(error), rowIndex));
	const filled = $derived(rows.filter((r) => String(r.value).trim() !== '').length);
	const warned = $derived((saved?.readings ?? []).filter((r) => r.warnings.length > 0));

	async function init() {
		const c = await loadCatalog();
		if (!c.ok) return void (loadError = c.error);
		catalog = c.data;
		rows = emptyRows(c.data, (session.user?.unit_prefs ?? {}) as UnitPrefs);
	}
	init();

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		const req = testToCreate(
			rows,
			measuredAt === initialMeasuredAt ? '' : fromLocalInput(measuredAt, tz),
			notes
		);
		rowIndex = req.rowIndex;
		busy = true;
		error = null;
		const r = await api<WaterTest>(
			'POST',
			`/tanks/${encodeURIComponent(id)}/water-tests`,
			req.body,
			fetch,
			{ 'Idempotency-Key': idempotencyKey }
		);
		busy = false;
		if (!r.ok) return void (error = r.error);
		if (r.data.readings.some((x) => x.warnings.length > 0)) saved = r.data;
		else await goto(resolve('/(app)/tanks/[id]/water-tests', { id }));
	}

	const unitsOf = (code: string) => catalog.find((c) => c.code === code)?.units ?? [];
</script>

<svelte:head><title>{m.wp_record()} · {m.app_name()}</title></svelte:head>

<p><a href={resolve('/(app)/tanks/[id]/water-tests', { id })}>← {m.wp_title()}</a></p>
<h1>{m.wp_record()}</h1>

{#if loadError}
	<Alert kind="error">
		{errorMessage(loadError)}
		<button type="button" class="secondary" onclick={init}>{m.retry()}</button>
	</Alert>
{:else if saved}
	<Alert kind="info">
		<p><strong>{m.wp_saved_check()}</strong></p>
		<ul>
			{#each warned as r (r.id)}
				<li>
					{parameterLabel(r.parameter)}: {valueText(r.input_value, r.input_unit, locale)} —
					{r.warnings.map(warningLabel).join('; ')}
				</li>
			{/each}
		</ul>
		<p>{m.wp_saved_check_hint()}</p>
	</Alert>
	<a class="button" href={resolve('/(app)/tanks/[id]/water-tests', { id })}>{m.wp_to_history()}</a>
{:else if catalog.length === 0}
	<p role="status">{m.loading()}</p>
{:else}
	<form onsubmit={submit} novalidate>
		{#if error && error.code !== 'validation_failed'}
			<Alert kind="error">{errorMessage(error)}</Alert>
		{:else if error}
			<Alert kind="error">{m.err_validation_failed()}</Alert>
		{/if}
		<Field
			label={m.wp_measured_at()}
			name="measured_at"
			type="datetime-local"
			bind:value={measuredAt}
			error={fields.measured_at}
			hint={m.wp_measured_at_hint()}
		/>
		{#if fields.readings}<p class="field-error">
				<span aria-hidden="true">⚠</span>
				{m.wp_need_one()}
			</p>{/if}

		<fieldset>
			<legend>{m.wp_values_legend()}</legend>
			<p class="hint">{m.wp_values_hint()}</p>
			{#each rows as row, i (row.parameter)}
				<div class="row">
					<Field
						label={parameterLabel(row.parameter)}
						name="value_{row.parameter}"
						inputmode="decimal"
						autocomplete="off"
						bind:value={row.value}
						error={fields[`${i}.value`]}
					/>
					{#if unitsOf(row.parameter).length > 1}
						<SelectField
							label={m.wp_unit_for({ parameter: parameterLabel(row.parameter) })}
							name="unit_{row.parameter}"
							bind:value={row.unit}
							options={unitsOf(row.parameter).map((u) => ({
								value: u.code,
								label: (unitLabel(u.code) || u.code) + (u.estimate ? ` (${m.wp_estimate()})` : '')
							}))}
							error={fields[`${i}.unit`]}
						/>
					{:else}
						<p class="unit" aria-hidden="true">{unitLabel(row.unit)}</p>
					{/if}
					<details class="more" open={!!(fields[`${i}.method`] || fields[`${i}.test_kit`])}>
						<summary>{m.wp_method_kit({ parameter: parameterLabel(row.parameter) })}</summary>
						<SelectField
							label={m.wp_method()}
							name="method_{row.parameter}"
							bind:value={row.method}
							options={[
								{ value: '', label: m.not_set() },
								...methods.map((x) => ({ value: x, label: methodLabel(x) }))
							]}
							error={fields[`${i}.method`]}
						/>
						<Field
							label={m.wp_test_kit()}
							name="kit_{row.parameter}"
							bind:value={row.testKit}
							optional
							maxlength={100}
							error={fields[`${i}.test_kit`]}
						/>
					</details>
				</div>
			{/each}
		</fieldset>

		<Field
			label={m.wc_notes_label()}
			name="notes"
			multiline
			optional
			bind:value={notes}
			error={fields.notes}
		/>
		<div class="actions">
			<button type="submit" disabled={busy || filled === 0}
				>{m.wp_save({ count: String(filled) })}</button
			>
			<a class="button secondary" href={resolve('/(app)/tanks/[id]/water-tests', { id })}
				>{m.cancel()}</a
			>
		</div>
	</form>
{/if}

<style>
	fieldset {
		border: 1px solid var(--border);
		border-radius: var(--radius);
		padding: 0.75rem 1rem;
		margin: 1rem 0;
	}
	.row {
		display: grid;
		grid-template-columns: minmax(8rem, 1fr) minmax(7rem, 10rem);
		gap: 0 0.75rem;
		align-items: start;
		border-top: 1px solid var(--border);
		padding-top: 0.5rem;
	}
	.row:first-of-type {
		border-top: 0;
	}
	.unit {
		margin: 2rem 0 0;
		color: var(--muted);
	}
	.more {
		grid-column: 1 / -1;
		margin: -0.25rem 0 0.5rem;
		font-size: 0.9rem;
	}
	.more[open] {
		padding-bottom: 0.25rem;
	}
</style>
