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
	import Alert from '#lib/components/Alert.svelte';
	import MaintenancePlanFormView from '#lib/components/MaintenancePlanForm.svelte';
	import {
		fromMaintenancePlan,
		maintenanceToPatch,
		type MaintenancePlanForm
	} from '#lib/equipment.js';
	import { errorMessage } from '#lib/errors.js';
	import { m } from '#lib/paraglide/messages.js';

	const id = $derived(page.params.id ?? '');
	const planId = $derived(page.params.planId ?? '');

	let original = $state<MaintenancePlan | null>(null);
	let form = $state<MaintenancePlanForm | null>(null);
	let devices = $state<Equipment[]>([]);
	let loading = $state(true);
	let loadError = $state<ApiError | null>(null);
	let busy = $state(false);
	let error = $state<ApiError | null>(null);

	const back = () => goto(resolve('/(app)/tanks/[id]/maintenance', { id }));

	$effect(() => {
		void planId;
		const tid = encodeURIComponent(id);
		// No single-plan GET: a tank has at most 100 plans, so find it in the list.
		Promise.all([
			api<Page<MaintenancePlan>>('GET', `/tanks/${tid}/maintenance-plans`),
			api<Page<Equipment>>('GET', `/tanks/${tid}/equipment`)
		]).then(([p, d]) => {
			loading = false;
			if (!p.ok) return void (loadError = p.error);
			original = p.data.items.find((x) => x.id === planId) ?? null;
			form = original ? fromMaintenancePlan(original) : null;
			if (d.ok) devices = d.data.items;
		});
	});

	async function save() {
		if (!original || !form) return;
		busy = true;
		error = null;
		const r = await api<MaintenancePlan>(
			'PATCH',
			`/maintenance-plans/${original.id}`,
			maintenanceToPatch(form, original)
		);
		busy = false;
		if (!r.ok) return void (error = r.error);
		await back();
	}
</script>

<svelte:head><title>{m.mt_plan_edit_title()} · {m.app_name()}</title></svelte:head>

<p>
	<a href={resolve('/(app)/tanks/[id]/maintenance', { id })}>← {m.tank_section_maintenance()}</a>
</p>
<h1>{m.mt_plan_edit_title()}</h1>

{#if loading}
	<p role="status">{m.loading()}</p>
{:else if loadError}
	<Alert kind="error">{errorMessage(loadError)}</Alert>
{:else if !form}
	<p>{m.not_found_body()}</p>
{:else}
	<MaintenancePlanFormView
		bind:form
		{devices}
		{error}
		{busy}
		submitLabel={m.save()}
		onsubmit={save}
		oncancel={back}
	/>
{/if}
