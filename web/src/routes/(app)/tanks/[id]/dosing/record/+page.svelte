<!--
	Record a dose given without a plan. An Idempotency-Key per form makes a retried submit
	(double click, flaky network) record the dose once. Unusually large doses or units ask for
	confirmation first (heuristic, never blocking; docs/open-items.md DS5).
-->
<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import {
		api,
		type ApiError,
		type DoseEvent,
		type DoseUnit,
		type DosingPlan,
		type DosingProduct,
		type Page
	} from '#lib/api/client.js';
	import Alert from '#lib/components/Alert.svelte';
	import ConfirmDialog from '#lib/components/ConfirmDialog.svelte';
	import Field from '#lib/components/Field.svelte';
	import SelectField from '#lib/components/SelectField.svelte';
	import { adHocWarning, unitLabel, unitsForForm } from '#lib/dosing.js';
	import { errorMessage, fieldErrors } from '#lib/errors.js';
	import { m } from '#lib/paraglide/messages.js';
	import { session } from '#lib/session.svelte.js';
	import { decimalInput } from '#lib/tank-form.js';
	import { fromLocalInput, toLocalInput } from '#lib/water-changes.js';

	const id = $derived(page.params.id ?? '');
	const tz = session.user?.timezone ?? 'Asia/Ho_Chi_Minh';

	let products = $state<DosingProduct[]>([]);
	let plans = $state<DosingPlan[]>([]);
	let loadError = $state<ApiError | null>(null);

	let productId = $state('');
	let amount = $state('');
	let unit = $state<DoseUnit>('ml');
	let at = $state(toLocalInput(new Date().toISOString(), tz));
	let notes = $state('');
	let busy = $state(false);
	let error = $state<ApiError | null>(null);
	let confirmOpen = $state(false);
	const fe = $derived(fieldErrors(error));
	const idempotencyKey = crypto.randomUUID();

	const product = $derived(products.find((p) => p.id === productId));
	const units = $derived(unitsForForm(product?.form));
	const warning = $derived(productId ? adHocWarning(productId, amount, unit, plans) : null);

	$effect(() => {
		if (product) unit = product.default_unit;
	});

	$effect(() => {
		Promise.all([
			api<Page<DosingProduct>>('GET', '/dosing-products'),
			api<Page<DosingPlan>>('GET', `/tanks/${encodeURIComponent(id)}/dosing-plans`)
		]).then(([p, pl]) => {
			if (!p.ok) loadError = p.error;
			else products = p.data.items;
			if (pl.ok) plans = pl.data.items;
		});
	});

	const back = () => goto(resolve('/(app)/tanks/[id]/dosing', { id }));

	function submit() {
		if (warning) confirmOpen = true;
		else save();
	}

	async function save() {
		busy = true;
		error = null;
		const r = await api<DoseEvent>(
			'POST',
			`/tanks/${encodeURIComponent(id)}/dosing-events`,
			{
				product_id: productId,
				amount: decimalInput(amount),
				amount_unit: unit,
				executed_at: fromLocalInput(at, tz) || undefined,
				notes: notes.trim() || null
			},
			fetch,
			{ 'Idempotency-Key': idempotencyKey }
		);
		busy = false;
		confirmOpen = false;
		if (!r.ok) {
			error = r.error;
			return;
		}
		await back();
	}
</script>

<svelte:head><title>{m.dose_record_ad_hoc()} · {m.app_name()}</title></svelte:head>

<p><a href={resolve('/(app)/tanks/[id]/dosing', { id })}>← {m.dose_title()}</a></p>
<h1>{m.dose_record_ad_hoc()}</h1>

{#if loadError}
	<Alert kind="error">{errorMessage(loadError)}</Alert>
{:else}
	<form
		novalidate
		onsubmit={(e) => {
			e.preventDefault();
			submit();
		}}
	>
		{#if error}
			<Alert kind="error">
				{error.code === 'validation_failed' ? m.err_validation_failed() : errorMessage(error)}
			</Alert>
		{/if}
		{#if products.length === 0}
			<Alert kind="info">{m.dose_no_products()}</Alert>
		{/if}

		<SelectField
			label={m.dose_product_label()}
			name="product_id"
			bind:value={productId}
			options={[
				{ value: '', label: m.dose_product_choose() },
				...products.map((p) => ({
					value: p.id,
					label: p.brand ? `${p.name} (${p.brand})` : p.name
				}))
			]}
			error={fe.product_id}
		/>
		<p class="hint"><a href={resolve('/(app)/dosing/products')}>{m.dose_products_manage()}</a></p>

		<Field
			label={m.dose_amount_label()}
			name="amount"
			inputmode="decimal"
			required
			bind:value={amount}
			error={fe.amount}
		/>
		<SelectField
			label={m.dose_unit_label()}
			name="amount_unit"
			bind:value={unit}
			options={units.map((u) => ({ value: u, label: unitLabel(u) }))}
			error={fe.amount_unit}
		/>
		{#if unit === 'drops'}<p class="hint">{m.dose_drops_hint()}</p>{/if}
		{#if product?.form === 'powder' && (unit === 'ml' || unit === 'l')}
			<p class="hint">{m.dose_powder_solution_hint()}</p>
		{/if}

		<Field
			label={m.dose_when_label()}
			name="executed_at"
			type="datetime-local"
			bind:value={at}
			hint={m.rec_timezone_note({ tz })}
			error={fe.executed_at}
		/>
		<Field
			label={m.wc_notes_label()}
			name="notes"
			optional
			multiline
			bind:value={notes}
			error={fe.notes}
		/>

		<div class="actions">
			<button type="submit" disabled={busy}>{busy ? m.saving() : m.dose_record()}</button>
			<button type="button" class="secondary" onclick={back}>{m.cancel()}</button>
		</div>
	</form>

	<ConfirmDialog
		bind:open={confirmOpen}
		title={m.dose_check_title()}
		confirmLabel={m.dose_record()}
		{busy}
		onconfirm={save}
	>
		<p>{warning === 'over_plan' ? m.dose_check_over_plan() : m.dose_check_unusual_unit()}</p>
	</ConfirmDialog>
{/if}
