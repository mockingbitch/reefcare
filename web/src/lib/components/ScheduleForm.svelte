<!-- Create/edit form for a recurring water-change schedule. -->
<script lang="ts">
	import type { ApiError } from '#lib/api/client.js';
	import { errorMessage, fieldErrors } from '#lib/errors.js';
	import { m } from '#lib/paraglide/messages.js';
	import type { ScheduleForm } from '#lib/schedule-form.js';
	import Alert from './Alert.svelte';
	import Field from './Field.svelte';
	import RecurrenceFields from './RecurrenceFields.svelte';

	type Props = {
		form: ScheduleForm;
		error: ApiError | null;
		busy: boolean;
		submitLabel: string;
		onsubmit: () => void;
		oncancel: () => void;
	};

	let { form = $bindable(), error, busy, submitLabel, onsubmit, oncancel }: Props = $props();

	const fe = $derived(fieldErrors(error));
</script>

<form
	novalidate
	onsubmit={(e) => {
		e.preventDefault();
		onsubmit();
	}}
>
	{#if error && error.code !== 'validation_failed'}
		<Alert kind="error">{errorMessage(error)}</Alert>
	{:else if error}
		<Alert kind="error">{m.err_validation_failed()}</Alert>
	{/if}

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
		label={m.wc_planned_volume_label()}
		name="planned_volume_l"
		inputmode="decimal"
		required
		bind:value={form.planned_volume_l}
		error={fe.planned_volume_l}
	/>
	<Field
		label={m.field_salt_mix()}
		name="salt_mix"
		optional
		bind:value={form.salt_mix}
		error={fe.salt_mix}
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
