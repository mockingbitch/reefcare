<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { api, type ApiError, type Equipment } from '#lib/api/client.js';
	import EquipmentFormView from '#lib/components/EquipmentForm.svelte';
	import { emptyEquipmentForm, equipmentToCreate } from '#lib/equipment.js';
	import { m } from '#lib/paraglide/messages.js';
	import { session } from '#lib/session.svelte.js';
	import { fromLocalInput, toLocalInput } from '#lib/water-changes.js';

	const id = $derived(page.params.id ?? '');
	const tz = session.user?.timezone ?? 'Asia/Ho_Chi_Minh';

	let form = $state(emptyEquipmentForm(toLocalInput(new Date().toISOString(), tz)));
	let busy = $state(false);
	let error = $state<ApiError | null>(null);

	async function create() {
		busy = true;
		error = null;
		const r = await api<Equipment>(
			'POST',
			`/tanks/${encodeURIComponent(id)}/equipment`,
			equipmentToCreate(form, fromLocalInput(form.installed_at, tz))
		);
		busy = false;
		if (!r.ok) {
			error = r.error;
			return;
		}
		await goto(
			resolve('/(app)/tanks/[id]/equipment/[equipmentId]', { id, equipmentId: r.data.id })
		);
	}
</script>

<svelte:head><title>{m.eq_add()} · {m.app_name()}</title></svelte:head>

<p><a href={resolve('/(app)/tanks/[id]/equipment', { id })}>← {m.eq_title()}</a></p>
<h1>{m.eq_add()}</h1>
<EquipmentFormView
	bind:form
	creating
	timeZone={tz}
	{error}
	{busy}
	submitLabel={m.eq_add()}
	onsubmit={create}
	oncancel={() => goto(resolve('/(app)/tanks/[id]/equipment', { id }))}
/>
