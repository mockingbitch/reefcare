<!--
	Records that a maintenance plan's task was done. Each opening gets its own
	Idempotency-Key, so a double submit records the work once.
-->
<script lang="ts">
	import {
		api,
		type ApiError,
		type MaintenanceEvent,
		type MaintenancePlan
	} from '#lib/api/client.js';
	import { errorMessage, fieldErrors } from '#lib/errors.js';
	import { m } from '#lib/paraglide/messages.js';
	import { fromLocalInput, toLocalInput } from '#lib/water-changes.js';
	import Alert from './Alert.svelte';
	import ConfirmDialog from './ConfirmDialog.svelte';
	import Field from './Field.svelte';

	type Props = {
		open: boolean;
		plan: MaintenancePlan | null;
		timeZone: string;
		ondone: () => void;
	};
	let { open = $bindable(false), plan, timeZone, ondone }: Props = $props();

	let performedAt = $state('');
	let notes = $state('');
	let key = '';
	let busy = $state(false);
	let error = $state<ApiError | null>(null);
	const fe = $derived(fieldErrors(error));

	let openedFor = '';
	$effect(() => {
		if (!open || !plan || openedFor === plan.id) return;
		openedFor = plan.id;
		performedAt = toLocalInput(new Date().toISOString(), timeZone);
		notes = '';
		key = crypto.randomUUID();
		error = null;
	});
	$effect(() => {
		if (!open) openedFor = '';
	});

	async function submit() {
		if (!plan) return;
		busy = true;
		error = null;
		const r = await api<MaintenanceEvent>(
			'POST',
			`/maintenance-plans/${plan.id}/events`,
			{
				performed_at: fromLocalInput(performedAt, timeZone) || undefined,
				notes: notes.trim() || null
			},
			fetch,
			{ 'Idempotency-Key': key }
		);
		busy = false;
		if (!r.ok) {
			error = r.error;
			return;
		}
		open = false;
		ondone();
	}
</script>

<ConfirmDialog
	bind:open
	title={plan ? m.mt_done_title({ title: plan.title }) : ''}
	confirmLabel={m.mt_done()}
	{busy}
	onconfirm={submit}
>
	{#if error && error.code !== 'validation_failed'}
		<Alert kind="error">{errorMessage(error)}</Alert>
	{/if}
	<Field
		label={m.mt_performed_at_label()}
		name="performed_at"
		type="datetime-local"
		bind:value={performedAt}
		hint={m.rec_timezone_note({ tz: timeZone })}
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
</ConfirmDialog>
