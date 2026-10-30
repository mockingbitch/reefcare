<!-- Record maintenance done without a plan (Idempotency-Key per form). -->
<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import {
		api,
		type ApiError,
		type Equipment,
		type MaintenanceEvent,
		type Page
	} from '#lib/api/client.js';
	import Alert from '#lib/components/Alert.svelte';
	import Field from '#lib/components/Field.svelte';
	import SelectField from '#lib/components/SelectField.svelte';
	import { errorMessage, fieldErrors } from '#lib/errors.js';
	import { m } from '#lib/paraglide/messages.js';
	import { session } from '#lib/session.svelte.js';
	import { fromLocalInput, toLocalInput } from '#lib/water-changes.js';

	const id = $derived(page.params.id ?? '');
	const tz = session.user?.timezone ?? 'Asia/Ho_Chi_Minh';

	let devices = $state<Equipment[]>([]);
	let title = $state('');
	let equipmentId = $state('');
	let performedAt = $state(toLocalInput(new Date().toISOString(), tz));
	let notes = $state('');
	let busy = $state(false);
	let error = $state<ApiError | null>(null);
	const fe = $derived(fieldErrors(error));
	const idempotencyKey = crypto.randomUUID();

	// Past work on a removed device may still be recorded, so list removed devices too.
	$effect(() => {
		api<Page<Equipment>>('GET', `/tanks/${encodeURIComponent(id)}/equipment?include=removed`).then(
			(r) => {
				if (r.ok) devices = r.data.items;
			}
		);
	});

	const back = () => goto(resolve('/(app)/tanks/[id]/maintenance', { id }));

	async function save() {
		busy = true;
		error = null;
		const r = await api<MaintenanceEvent>(
			'POST',
			`/tanks/${encodeURIComponent(id)}/maintenance-events`,
			{
				title: title.trim(),
				equipment_id: equipmentId || null,
				performed_at: fromLocalInput(performedAt, tz) || undefined,
				notes: notes.trim() || null
			},
			fetch,
			{ 'Idempotency-Key': idempotencyKey }
		);
		busy = false;
		if (!r.ok) return void (error = r.error);
		await back();
	}
</script>

<svelte:head><title>{m.mt_record()} · {m.app_name()}</title></svelte:head>

<p>
	<a href={resolve('/(app)/tanks/[id]/maintenance', { id })}>← {m.tank_section_maintenance()}</a>
</p>
<h1>{m.mt_record()}</h1>

<form
	novalidate
	onsubmit={(e) => {
		e.preventDefault();
		save();
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
		bind:value={title}
		hint={m.mt_title_hint()}
		error={fe.title}
	/>
	<SelectField
		label={m.mt_device_label()}
		name="equipment_id"
		bind:value={equipmentId}
		options={[
			{ value: '', label: m.mt_device_none() },
			...devices.map((d) => ({
				value: d.id,
				label: d.removed_at ? `${d.name} (${m.eq_removed_badge()})` : d.name
			}))
		]}
		error={fe.equipment_id}
	/>
	<Field
		label={m.mt_performed_at_label()}
		name="performed_at"
		type="datetime-local"
		bind:value={performedAt}
		hint={m.rec_timezone_note({ tz })}
		error={fe.performed_at}
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
		<button type="submit" disabled={busy}>{busy ? m.saving() : m.save()}</button>
		<button type="button" class="secondary" onclick={back}>{m.cancel()}</button>
	</div>
</form>
