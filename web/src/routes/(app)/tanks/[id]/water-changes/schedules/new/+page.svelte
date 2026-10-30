<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { api, type ApiError, type WaterChangeSchedule } from '#lib/api/client.js';
	import ScheduleFormView from '#lib/components/ScheduleForm.svelte';
	import { today } from '#lib/format.js';
	import { m } from '#lib/paraglide/messages.js';
	import { emptyScheduleForm, toCreate } from '#lib/schedule-form.js';
	import { session } from '#lib/session.svelte.js';

	const id = $derived(page.params.id ?? '');
	const tz = session.user?.timezone ?? 'Asia/Ho_Chi_Minh';

	let form = $state(emptyScheduleForm(today(tz), tz));
	let busy = $state(false);
	let error = $state<ApiError | null>(null);

	const back = () => goto(resolve('/(app)/tanks/[id]/water-changes', { id }));

	async function create() {
		busy = true;
		error = null;
		const r = await api<WaterChangeSchedule>(
			'POST',
			`/tanks/${encodeURIComponent(id)}/water-change-schedules`,
			toCreate(form)
		);
		busy = false;
		if (!r.ok) {
			error = r.error;
			return;
		}
		await back();
	}
</script>

<svelte:head><title>{m.wc_schedule_new_title()} · {m.app_name()}</title></svelte:head>

<p><a href={resolve('/(app)/tanks/[id]/water-changes', { id })}>← {m.wc_title()}</a></p>
<h1>{m.wc_schedule_new_title()}</h1>
<ScheduleFormView
	bind:form
	{error}
	{busy}
	submitLabel={m.wc_schedule_create_submit()}
	onsubmit={create}
	oncancel={back}
/>
