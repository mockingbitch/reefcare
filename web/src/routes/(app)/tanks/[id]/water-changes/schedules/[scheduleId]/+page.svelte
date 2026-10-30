<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { api, type ApiError, type Page, type WaterChangeSchedule } from '#lib/api/client.js';
	import Alert from '#lib/components/Alert.svelte';
	import ScheduleFormView from '#lib/components/ScheduleForm.svelte';
	import { errorMessage } from '#lib/errors.js';
	import { m } from '#lib/paraglide/messages.js';
	import { fromSchedule, toPatch, type ScheduleForm } from '#lib/schedule-form.js';

	const id = $derived(page.params.id ?? '');
	const scheduleId = $derived(page.params.scheduleId ?? '');

	let original = $state<WaterChangeSchedule | null>(null);
	let form = $state<ScheduleForm | null>(null);
	let loading = $state(true);
	let loadError = $state<ApiError | null>(null);
	let busy = $state(false);
	let error = $state<ApiError | null>(null);

	const back = () => goto(resolve('/(app)/tanks/[id]/water-changes', { id }));

	async function load() {
		loading = true;
		// A tank has few schedules and there is no single-schedule GET; find it in the list.
		const r = await api<Page<WaterChangeSchedule>>(
			'GET',
			`/tanks/${encodeURIComponent(id)}/water-change-schedules`
		);
		loading = false;
		if (!r.ok) {
			loadError = r.error;
			return;
		}
		original = r.data.items.find((s) => s.id === scheduleId) ?? null;
		form = original ? fromSchedule(original) : null;
	}

	$effect(() => {
		void scheduleId;
		load();
	});

	async function save() {
		if (!original || !form) return;
		busy = true;
		error = null;
		const r = await api<WaterChangeSchedule>(
			'PATCH',
			`/water-change-schedules/${original.id}`,
			toPatch(form, original)
		);
		busy = false;
		if (!r.ok) {
			error = r.error;
			return;
		}
		await back();
	}
</script>

<svelte:head><title>{m.wc_schedule_edit_title()} · {m.app_name()}</title></svelte:head>

<p><a href={resolve('/(app)/tanks/[id]/water-changes', { id })}>← {m.wc_title()}</a></p>
<h1>{m.wc_schedule_edit_title()}</h1>

{#if loading}
	<p role="status">{m.loading()}</p>
{:else if loadError}
	<Alert kind="error">{errorMessage(loadError)}</Alert>
{:else if !form}
	<p>{m.not_found_body()}</p>
{:else}
	<ScheduleFormView
		bind:form
		{error}
		{busy}
		submitLabel={m.save()}
		onsubmit={save}
		oncancel={back}
	/>
{/if}
