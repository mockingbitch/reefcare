<!-- Create/edit form for an interval-based maintenance plan. -->
<script lang="ts">
	import type { ApiError, Equipment } from '#lib/api/client.js';
	import type { MaintenancePlanForm } from '#lib/equipment.js';
	import { errorMessage, fieldErrors } from '#lib/errors.js';
	import { m } from '#lib/paraglide/messages.js';
	import Alert from './Alert.svelte';
	import Field from './Field.svelte';
	import SelectField from './SelectField.svelte';

	type Props = {
		form: MaintenancePlanForm;
		/** Installed devices of the tank (removed ones can't get an active plan). */
		devices: Equipment[];
		error: ApiError | null;
		busy: boolean;
		submitLabel: string;
		onsubmit: () => void;
		oncancel: () => void;
	};

	let {
		form = $bindable(),
		devices,
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
	<Field
		label={m.mt_title_label()}
		name="title"
		required
		bind:value={form.title}
		hint={m.mt_title_hint()}
		error={fe.title}
	/>
	<Field
		label={m.mt_interval_label()}
		name="interval_days"
		type="number"
		min="1"
		max="730"
		required
		bind:value={form.interval_days}
		error={fe.interval_days}
	/>
	<Field
		label={m.mt_starts_on_label()}
		name="starts_on"
		type="date"
		required
		bind:value={form.starts_on}
		hint={m.rec_timezone_note({ tz: form.timezone })}
		error={fe.starts_on ?? fe.timezone}
	/>
	<SelectField
		label={m.mt_device_label()}
		name="equipment_id"
		bind:value={form.equipment_id}
		options={[
			{ value: '', label: m.mt_device_none() },
			...devices.map((d) => ({ value: d.id, label: d.name }))
		]}
		error={fe.equipment_id}
	/>
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
