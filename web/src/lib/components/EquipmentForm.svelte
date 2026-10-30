<!-- Create/edit form for a device. `installed_at` is set once, at creation. -->
<script lang="ts">
	import type { ApiError } from '#lib/api/client.js';
	import { categories, categoryLabel, type EquipmentForm } from '#lib/equipment.js';
	import { errorMessage, fieldErrors } from '#lib/errors.js';
	import { m } from '#lib/paraglide/messages.js';
	import Alert from './Alert.svelte';
	import Field from './Field.svelte';
	import SelectField from './SelectField.svelte';

	type Props = {
		form: EquipmentForm;
		creating: boolean;
		timeZone: string;
		error: ApiError | null;
		busy: boolean;
		submitLabel: string;
		onsubmit: () => void;
		oncancel: () => void;
	};

	let {
		form = $bindable(),
		creating,
		timeZone,
		error,
		busy,
		submitLabel,
		onsubmit,
		oncancel
	}: Props = $props();
	const fe = $derived(fieldErrors(error));
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
	<SelectField
		label={m.eq_category_label()}
		name="category"
		bind:value={form.category}
		options={categories.map((c) => ({ value: c, label: categoryLabel(c) }))}
		error={fe.category}
	/>
	<Field label={m.eq_name_label()} name="name" required bind:value={form.name} error={fe.name} />
	<Field
		label={m.eq_manufacturer_label()}
		name="manufacturer"
		optional
		bind:value={form.manufacturer}
		error={fe.manufacturer}
	/>
	<Field
		label={m.eq_model_label()}
		name="model"
		optional
		bind:value={form.model}
		error={fe.model}
	/>
	<Field
		label={m.eq_serial_label()}
		name="serial_number"
		optional
		bind:value={form.serial_number}
		error={fe.serial_number}
	/>
	<Field
		label={m.eq_purchased_label()}
		name="purchased_on"
		type="date"
		optional
		bind:value={form.purchased_on}
		error={fe.purchased_on}
	/>
	<Field
		label={m.eq_warranty_label()}
		name="warranty_until"
		type="date"
		optional
		bind:value={form.warranty_until}
		error={fe.warranty_until}
	/>
	{#if creating}
		<Field
			label={m.eq_installed_label()}
			name="installed_at"
			type="datetime-local"
			bind:value={form.installed_at}
			hint={m.eq_installed_hint({ tz: timeZone })}
			error={fe.installed_at}
		/>
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
