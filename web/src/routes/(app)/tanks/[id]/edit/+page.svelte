<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { api, type ApiError, type Tank } from '#lib/api/client.js';
	import Alert from '#lib/components/Alert.svelte';
	import TankFormView from '#lib/components/TankForm.svelte';
	import { errorMessage } from '#lib/errors.js';
	import { m } from '#lib/paraglide/messages.js';
	import { session } from '#lib/session.svelte.js';
	import { fromTank, toPatch, type TankForm } from '#lib/tank-form.js';

	const id = $derived(page.params.id ?? '');
	let tank = $state<Tank | null>(null);
	let form = $state<TankForm | null>(null);
	let loadError = $state<ApiError | null>(null);
	let error = $state<ApiError | null>(null);
	let busy = $state(false);

	async function load() {
		loadError = null;
		const r = await api<Tank>('GET', `/tanks/${encodeURIComponent(id)}`);
		if (!r.ok) {
			loadError = r.error;
			return;
		}
		tank = r.data;
		form = fromTank(r.data);
	}

	$effect(() => {
		void id;
		load();
	});

	async function save() {
		if (!tank || !form) return;
		busy = true;
		error = null;
		const r = await api<Tank>(
			'PATCH',
			`/tanks/${encodeURIComponent(tank.id)}`,
			toPatch(form, tank)
		);
		busy = false;
		if (!r.ok) {
			error = r.error;
			return;
		}
		await goto(resolve('/(app)/tanks/[id]', { id: r.data.id }));
	}
</script>

<svelte:head><title>{m.tank_edit_title()} · {m.app_name()}</title></svelte:head>

<h1>{m.tank_edit_title()}</h1>

{#if loadError}
	<Alert kind="error">
		{errorMessage(loadError)}
		<button type="button" class="secondary" onclick={load}>{m.retry()}</button>
	</Alert>
{:else if !form || !tank}
	<p role="status">{m.loading()}</p>
{:else if tank.access !== 'owner' || tank.status === 'archived'}
	<Alert kind="info">{tank.status === 'archived' ? m.archived_notice() : m.viewer_notice()}</Alert>
{:else}
	{#if error?.code === 'version_conflict'}
		<p><button type="button" class="secondary" onclick={load}>{m.retry()}</button></p>
	{/if}
	<TankFormView
		bind:form
		{error}
		{busy}
		submitLabel={m.tank_edit_submit()}
		timeZone={session.user?.timezone}
		onsubmit={save}
		oncancel={() => goto(resolve('/(app)/tanks/[id]', { id }))}
	/>
{/if}
