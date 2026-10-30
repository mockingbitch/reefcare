<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { api, type ApiError, type Equipment } from '#lib/api/client.js';
	import Alert from '#lib/components/Alert.svelte';
	import EquipmentFormView from '#lib/components/EquipmentForm.svelte';
	import { equipmentToPatch, fromEquipment, type EquipmentForm } from '#lib/equipment.js';
	import { errorMessage } from '#lib/errors.js';
	import { m } from '#lib/paraglide/messages.js';
	import { session } from '#lib/session.svelte.js';

	const id = $derived(page.params.id ?? '');
	const equipmentId = $derived(page.params.equipmentId ?? '');
	const tz = session.user?.timezone ?? 'Asia/Ho_Chi_Minh';

	let original = $state<Equipment | null>(null);
	let form = $state<EquipmentForm | null>(null);
	let loadError = $state<ApiError | null>(null);
	let busy = $state(false);
	let error = $state<ApiError | null>(null);

	const back = () =>
		goto(resolve('/(app)/tanks/[id]/equipment/[equipmentId]', { id, equipmentId }));

	$effect(() => {
		api<Equipment>('GET', `/equipment/${encodeURIComponent(equipmentId)}`).then((r) => {
			if (!r.ok) return void (loadError = r.error);
			original = r.data;
			form = fromEquipment(r.data);
		});
	});

	async function save() {
		if (!original || !form) return;
		busy = true;
		error = null;
		const r = await api<Equipment>(
			'PATCH',
			`/equipment/${original.id}`,
			equipmentToPatch(form, original)
		);
		busy = false;
		if (!r.ok) {
			error = r.error;
			return;
		}
		await back();
	}
</script>

<svelte:head><title>{m.eq_edit_title()} · {m.app_name()}</title></svelte:head>

<p>
	<a href={resolve('/(app)/tanks/[id]/equipment/[equipmentId]', { id, equipmentId })}
		>← {original?.name ?? m.eq_title()}</a
	>
</p>
<h1>{m.eq_edit_title()}</h1>

{#if loadError}
	<Alert kind="error">{errorMessage(loadError)}</Alert>
{:else if form}
	<EquipmentFormView
		bind:form
		creating={false}
		timeZone={tz}
		{error}
		{busy}
		submitLabel={m.save()}
		onsubmit={save}
		oncancel={back}
	/>
{:else}
	<p role="status">{m.loading()}</p>
{/if}
