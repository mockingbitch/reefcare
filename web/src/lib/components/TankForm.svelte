<!-- Create/edit form for a tank. The parent owns submission; this renders fields and errors. -->
<script lang="ts">
	import type { ApiError } from '#lib/api/client.js';
	import Alert from '#lib/components/Alert.svelte';
	import Field from '#lib/components/Field.svelte';
	import SelectField from '#lib/components/SelectField.svelte';
	import { errorMessage, fieldErrors } from '#lib/errors.js';
	import { m } from '#lib/paraglide/messages.js';
	import {
		clientProblems,
		editableStatuses,
		systemBelowDisplay,
		tankTypes,
		type TankForm
	} from '#lib/tank-form.js';
	import { tankTypeLabel, statusLabel } from '#lib/labels.js';
	import { today } from '#lib/format.js';

	type Props = {
		form: TankForm;
		error: ApiError | null;
		busy: boolean;
		submitLabel: string;
		timeZone?: string;
		onsubmit: () => void;
		oncancel?: () => void;
	};
	let {
		form = $bindable(),
		error,
		busy,
		submitLabel,
		timeZone,
		onsubmit,
		oncancel
	}: Props = $props();

	let localProblems = $state<Record<string, string>>({});
	const fields = $derived({ ...fieldErrors(error), ...localProblems });
	const warnVolume = $derived(systemBelowDisplay(form));

	function submit(e: SubmitEvent) {
		e.preventDefault();
		const problems = clientProblems(form);
		localProblems = problems.dimensions_cm ? { dimensions_cm: m.f_dimensions_all_or_none() } : {};
		if (Object.keys(problems).length === 0) onsubmit();
	}

	const typeOptions = tankTypes.map((t) => ({ value: t, label: tankTypeLabel(t) }));
	const statusOptions = editableStatuses.map((s) => ({ value: s, label: statusLabel(s) }));
</script>

<form class="card" onsubmit={submit} novalidate>
	{#if error}<Alert kind="error">{errorMessage(error)}</Alert>{/if}

	<Field
		label={m.field_name()}
		name="name"
		maxlength={80}
		required
		bind:value={form.name}
		error={fields.name}
	/>
	<SelectField
		label={m.field_tank_type()}
		name="tank_type"
		options={typeOptions}
		bind:value={form.tank_type}
		error={fields.tank_type}
	/>
	<SelectField
		label={m.field_status()}
		name="status"
		options={statusOptions}
		bind:value={form.status}
		error={fields.status}
	/>

	<fieldset>
		<legend>{m.tank_volumes()}</legend>
		<Field
			label={m.field_display_volume()}
			name="display_volume_l"
			inputmode="decimal"
			required
			bind:value={form.display_volume_l}
			error={fields.display_volume_l}
		/>
		<Field
			label={m.field_sump_volume()}
			name="sump_volume_l"
			inputmode="decimal"
			optional
			bind:value={form.sump_volume_l}
			error={fields.sump_volume_l}
		/>
		<Field
			label={m.field_system_volume()}
			name="system_volume_l"
			inputmode="decimal"
			required
			hint={m.system_volume_hint()}
			bind:value={form.system_volume_l}
			error={fields.system_volume_l}
		/>
		{#if warnVolume}<Alert kind="warning">{m.warn_system_below_display()}</Alert>{/if}
	</fieldset>

	<fieldset aria-describedby={fields.dimensions_cm ? 'dims-error' : undefined}>
		<legend>{m.field_dimensions()} <span class="muted">({m.optional()})</span></legend>
		<div class="grid-3">
			<Field
				label={m.field_length()}
				name="length"
				inputmode="decimal"
				bind:value={form.length}
				error={fields['dimensions_cm.length']}
			/>
			<Field
				label={m.field_width()}
				name="width"
				inputmode="decimal"
				bind:value={form.width}
				error={fields['dimensions_cm.width']}
			/>
			<Field
				label={m.field_height()}
				name="height"
				inputmode="decimal"
				bind:value={form.height}
				error={fields['dimensions_cm.height']}
			/>
		</div>
		{#if fields.dimensions_cm}
			<p id="dims-error" class="field-error">
				<span aria-hidden="true">⚠</span>
				{fields.dimensions_cm}
			</p>
		{/if}
	</fieldset>

	<Field
		label={m.field_setup_date()}
		name="setup_date"
		type="date"
		max={today(timeZone)}
		optional
		bind:value={form.setup_date}
		error={fields.setup_date}
	/>
	<Field
		label={m.field_salt_mix()}
		name="salt_mix"
		maxlength={100}
		optional
		bind:value={form.salt_mix}
		error={fields.salt_mix}
	/>
	<Field
		label={m.field_water_source()}
		name="water_source"
		maxlength={100}
		optional
		bind:value={form.water_source}
		error={fields.water_source}
	/>
	<Field
		label={m.field_description()}
		name="description"
		multiline
		optional
		bind:value={form.description}
		error={fields.description}
	/>

	<div class="actions">
		<button type="submit" disabled={busy}>{busy ? m.saving() : submitLabel}</button>
		{#if oncancel}<button type="button" class="secondary" onclick={oncancel}>{m.cancel()}</button
			>{/if}
	</div>
</form>
