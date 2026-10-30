<!-- Create/edit form for a livestock entry. The quantity is only set when adding. -->
<script lang="ts">
	import type { ApiError, Species } from '#lib/api/client.js';
	import { errorMessage, fieldErrors } from '#lib/errors.js';
	import {
		coralTypeLabel,
		coralTypes,
		livestockCategories,
		livestockCategoryLabel,
		livestockFormLabel,
		livestockForms,
		quantityIsApproximate,
		speciesName,
		type EntryForm
	} from '#lib/livestock.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import Alert from './Alert.svelte';
	import Field from './Field.svelte';
	import SelectField from './SelectField.svelte';
	import SpeciesPicker from './SpeciesPicker.svelte';

	type Props = {
		form: EntryForm;
		creating: boolean;
		/** Name of the entry's current species (edit). */
		speciesLabel?: string;
		/** `datetime-local` value for added_at (create only). */
		addedAt?: string;
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
		speciesLabel = '',
		addedAt = $bindable(''),
		timeZone,
		error,
		busy,
		submitLabel,
		onsubmit,
		oncancel
	}: Props = $props();
	const fe = $derived(fieldErrors(error));
	const locale = getLocale();

	function picked(s: Species | null) {
		if (!s) return;
		form.category = s.category;
		if (s.coral_type) form.coral_type = s.coral_type;
		if (!form.display_name.trim()) form.display_name = speciesName(s, locale);
	}
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

	<SpeciesPicker bind:value={form.species_id} currentName={speciesLabel} onpick={picked} />
	{#if fe.species_id}<p class="field-error">
			<span aria-hidden="true">⚠</span>
			{fe.species_id}
		</p>{/if}

	<SelectField
		label={m.ls_category_label()}
		name="category"
		bind:value={form.category}
		options={livestockCategories.map((c) => ({ value: c, label: livestockCategoryLabel(c) }))}
		error={fe.category}
	/>
	<Field
		label={m.ls_display_name_label()}
		name="display_name"
		required
		bind:value={form.display_name}
		hint={m.ls_display_name_hint()}
		error={fe.display_name}
	/>
	{#if creating}
		<Field
			label={m.ls_quantity_label()}
			name="quantity"
			type="number"
			min="1"
			max="10000"
			required
			bind:value={form.quantity}
			hint={quantityIsApproximate(form.category)
				? m.ls_quantity_hint_approx()
				: m.ls_quantity_hint()}
			error={fe.quantity}
		/>
	{/if}
	<SelectField
		label={m.ls_form_label()}
		name="form"
		bind:value={form.form}
		options={[
			{ value: '', label: m.not_set() },
			...livestockForms.map((f) => ({ value: f, label: livestockFormLabel(f) }))
		]}
		error={fe.form}
	/>
	{#if form.category === 'coral'}
		<SelectField
			label={m.ls_coral_type_label()}
			name="coral_type"
			bind:value={form.coral_type}
			options={[
				{ value: '', label: m.not_set() },
				...coralTypes.map((c) => ({ value: c, label: coralTypeLabel(c) }))
			]}
			error={fe.coral_type}
		/>
	{/if}
	<SelectField
		label={m.ls_quarantine_label()}
		name="quarantine_status"
		bind:value={form.quarantine_status}
		options={[
			{ value: '', label: m.ls_quarantine_none() },
			{ value: 'in_quarantine', label: m.ls_quarantine_in() },
			{ value: 'completed', label: m.ls_quarantine_completed() }
		]}
		error={fe.quarantine_status}
	/>
	<Field
		label={m.ls_location_label()}
		name="location_note"
		optional
		bind:value={form.location_note}
		error={fe.location_note}
	/>
	<Field
		label={m.ls_source_label()}
		name="source_note"
		optional
		bind:value={form.source_note}
		error={fe.source_note}
	/>
	{#if creating}
		<Field
			label={m.ls_added_at_label()}
			name="added_at"
			type="datetime-local"
			bind:value={addedAt}
			hint={m.rec_timezone_note({ tz: timeZone })}
			error={fe.added_at}
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
