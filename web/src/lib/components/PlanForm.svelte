<!-- Create/edit form for a recurring dosing plan. -->
<script lang="ts">
	import { resolve } from '$app/paths';
	import type { ApiError, DosingProduct } from '#lib/api/client.js';
	import { unitLabel, unitsForForm, type PlanForm } from '#lib/dosing.js';
	import { errorMessage, fieldErrors } from '#lib/errors.js';
	import { m } from '#lib/paraglide/messages.js';
	import Alert from './Alert.svelte';
	import Field from './Field.svelte';
	import RecurrenceFields from './RecurrenceFields.svelte';
	import SelectField from './SelectField.svelte';

	type Props = {
		form: PlanForm;
		products: DosingProduct[];
		/** Editing: the product of an existing plan can't change. */
		productLocked?: boolean;
		error: ApiError | null;
		busy: boolean;
		submitLabel: string;
		onsubmit: () => void;
		oncancel: () => void;
	};

	let {
		form = $bindable(),
		products,
		productLocked = false,
		error,
		busy,
		submitLabel,
		onsubmit,
		oncancel
	}: Props = $props();

	const fe = $derived(fieldErrors(error));
	const product = $derived(products.find((p) => p.id === form.product_id));
	const units = $derived(unitsForForm(product?.form));

	// Picking a product proposes its default unit (and keeps the unit valid for its form).
	let lastProduct = '';
	$effect(() => {
		if (!product || product.id === lastProduct) return;
		const first = lastProduct === '';
		lastProduct = product.id;
		if (productLocked && first) return;
		if (!units.includes(form.amount_unit) || !first) form.amount_unit = product.default_unit;
	});
</script>

<form
	novalidate
	onsubmit={(e) => {
		e.preventDefault();
		onsubmit();
	}}
>
	{#if error}
		<Alert kind="error">
			{error.code === 'validation_failed' ? m.err_validation_failed() : errorMessage(error)}
		</Alert>
	{/if}

	{#if productLocked}
		<p><strong>{m.dose_product_label()}:</strong> {product?.name ?? ''}</p>
	{:else}
		<SelectField
			label={m.dose_product_label()}
			name="product_id"
			bind:value={form.product_id}
			options={[
				{ value: '', label: m.dose_product_choose() },
				...products.map((p) => ({
					value: p.id,
					label: p.brand ? `${p.name} (${p.brand})` : p.name
				}))
			]}
			error={fe.product_id}
		/>
		<p class="hint">
			<a href={resolve('/(app)/dosing/products')}>{m.dose_products_manage()}</a>
		</p>
	{/if}

	<div class="amount">
		<Field
			label={m.dose_amount_label()}
			name="amount"
			inputmode="decimal"
			required
			bind:value={form.amount}
			error={fe.amount}
		/>
		<SelectField
			label={m.dose_unit_label()}
			name="amount_unit"
			bind:value={form.amount_unit}
			options={units.map((u) => ({ value: u, label: unitLabel(u) }))}
			error={fe.amount_unit}
		/>
	</div>
	{#if form.amount_unit === 'drops'}
		<p class="hint">{m.dose_drops_hint()}</p>
	{/if}
	{#if product?.form === 'powder' && (form.amount_unit === 'ml' || form.amount_unit === 'l')}
		<p class="hint">{m.dose_powder_solution_hint()}</p>
	{/if}

	<SelectField
		label={m.dose_delivery_label()}
		name="delivery_method"
		bind:value={form.delivery_method}
		options={[
			{ value: 'manual', label: m.dose_delivery_manual() },
			{ value: 'dosing_pump', label: m.dose_delivery_pump() }
		]}
		hint={form.delivery_method === 'dosing_pump' ? m.dose_pump_hint() : undefined}
		error={fe.delivery_method}
	/>

	<Field
		label={m.wc_starts_on_label()}
		name="starts_on"
		type="date"
		required
		bind:value={form.starts_on}
		error={fe.starts_on}
	/>
	<Field
		label={m.wc_ends_on_label()}
		name="ends_on"
		type="date"
		optional
		bind:value={form.ends_on}
		error={fe.ends_on}
	/>

	{#if form.recurrence}
		<RecurrenceFields
			bind:form={form.recurrence}
			startsOn={form.starts_on}
			endsOn={form.ends_on}
			timeZone={form.timezone}
			error={fe.rrule ?? fe.timezone}
		/>
	{:else}
		<Alert kind="info">{m.rec_not_editable({ rule: form.rrule })}</Alert>
	{/if}

	<Field
		label={m.wc_notes_label()}
		name="notes"
		optional
		multiline
		bind:value={form.notes}
		error={fe.notes}
	/>

	<div class="actions">
		<button type="submit" disabled={busy}>{busy ? m.saving() : submitLabel}</button>
		<button type="button" class="secondary" onclick={oncancel}>{m.cancel()}</button>
	</div>
</form>

<style>
	.amount {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 0 1rem;
	}
</style>
