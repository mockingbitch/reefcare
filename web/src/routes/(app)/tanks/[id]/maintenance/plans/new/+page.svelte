<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import {
		api,
		type ApiError,
		type Equipment,
		type MaintenancePlan,
		type Page
	} from '#lib/api/client.js';
	import MaintenancePlanFormView from '#lib/components/MaintenancePlanForm.svelte';
	import { emptyMaintenanceForm, maintenanceToCreate } from '#lib/equipment.js';
	import { today } from '#lib/format.js';
	import { m } from '#lib/paraglide/messages.js';
	import { session } from '#lib/session.svelte.js';

	const id = $derived(page.params.id ?? '');
	const tz = session.user?.timezone ?? 'Asia/Ho_Chi_Minh';

	let form = $state(emptyMaintenanceForm(today(tz), tz));
	let devices = $state<Equipment[]>([]);
	let busy = $state(false);
	let error = $state<ApiError | null>(null);

	// Preselect a device when coming from its page (?equipment_id=…).
	$effect(() => {
		const pre = page.url.searchParams.get('equipment_id');
		if (pre) form.equipment_id = pre;
		api<Page<Equipment>>('GET', `/tanks/${encodeURIComponent(id)}/equipment`).then((r) => {
			if (r.ok) devices = r.data.items;
		});
	});

	const back = () => goto(resolve('/(app)/tanks/[id]/maintenance', { id }));

	async function create() {
		busy = true;
		error = null;
		const r = await api<MaintenancePlan>(
			'POST',
			`/tanks/${encodeURIComponent(id)}/maintenance-plans`,
			maintenanceToCreate(form)
		);
		busy = false;
		if (!r.ok) return void (error = r.error);
		await back();
	}
</script>

<svelte:head><title>{m.mt_plan_new()} · {m.app_name()}</title></svelte:head>

<p>
	<a href={resolve('/(app)/tanks/[id]/maintenance', { id })}>← {m.tank_section_maintenance()}</a>
</p>
<h1>{m.mt_plan_new()}</h1>
<MaintenancePlanFormView
	bind:form
	{devices}
	{error}
	{busy}
	submitLabel={m.mt_plan_create_submit()}
	onsubmit={create}
	oncancel={back}
/>
