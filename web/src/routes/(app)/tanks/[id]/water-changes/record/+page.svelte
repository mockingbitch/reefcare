<!-- Record a water change done without a schedule, or plan a one-time change. -->
<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { api, type ApiError, type WaterChange, type WaterChangeRecord } from '#lib/api/client.js';
	import Alert from '#lib/components/Alert.svelte';
	import Field from '#lib/components/Field.svelte';
	import SelectField from '#lib/components/SelectField.svelte';
	import { errorMessage, fieldErrors } from '#lib/errors.js';
	import { m } from '#lib/paraglide/messages.js';
	import { session } from '#lib/session.svelte.js';
	import { decimalInput } from '#lib/tank-form.js';
	import { fromLocalInput, toLocalInput } from '#lib/water-changes.js';

	const id = $derived(page.params.id ?? '');
	const tz = session.user?.timezone ?? 'Asia/Ho_Chi_Minh';

	let mode = $state<'done' | 'planned'>('done');
	let at = $state(toLocalInput(new Date().toISOString(), tz));
	let volume = $state('');
	let saltMix = $state('');
	let notes = $state('');
	let busy = $state(false);
	let error = $state<ApiError | null>(null);
	const fe = $derived(fieldErrors(error));

	const back = () => goto(resolve('/(app)/tanks/[id]/water-changes', { id }));

	async function save() {
		busy = true;
		error = null;
		const when = fromLocalInput(at, tz);
		const body: WaterChangeRecord =
			mode === 'done'
				? { completed_at: when, actual_volume_l: decimalInput(volume) }
				: { scheduled_for: when, planned_volume_l: decimalInput(volume) };
		body.salt_mix = saltMix.trim() || null;
		body.notes = notes.trim() || null;
		const r = await api<WaterChange>(
			'POST',
			`/tanks/${encodeURIComponent(id)}/water-changes`,
			body
		);
		busy = false;
		if (!r.ok) {
			error = r.error;
			return;
		}
		await back();
	}
</script>

<svelte:head><title>{m.wc_record_title()} · {m.app_name()}</title></svelte:head>

<p><a href={resolve('/(app)/tanks/[id]/water-changes', { id })}>← {m.wc_title()}</a></p>
<h1>{m.wc_record_title()}</h1>

<form
	novalidate
	onsubmit={(e) => {
		e.preventDefault();
		save();
	}}
>
	{#if error}
		<Alert kind="error">{errorMessage(error)}</Alert>
	{/if}

	<SelectField
		label={m.wc_record_mode_label()}
		name="mode"
		bind:value={mode}
		options={[
			{ value: 'done', label: m.wc_record_mode_done() },
			{ value: 'planned', label: m.wc_record_mode_planned() }
		]}
	/>
	<Field
		label={mode === 'done' ? m.wc_completed_at_label() : m.wc_scheduled_for_label()}
		name="at"
		type="datetime-local"
		required
		bind:value={at}
		hint={m.rec_timezone_note({ tz })}
		error={fe.completed_at ?? fe.scheduled_for}
	/>
	<Field
		label={mode === 'done' ? m.wc_actual_volume_label() : m.wc_planned_volume_label()}
		name="volume"
		inputmode="decimal"
		required
		bind:value={volume}
		error={fe.actual_volume_l ?? fe.planned_volume_l}
	/>
	<Field
		label={m.field_salt_mix()}
		name="salt_mix"
		optional
		bind:value={saltMix}
		error={fe.salt_mix}
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
