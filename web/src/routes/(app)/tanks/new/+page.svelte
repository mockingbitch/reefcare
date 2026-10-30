<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { api, type ApiError, type Tank } from '#lib/api/client.js';
	import TankFormView from '#lib/components/TankForm.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { session } from '#lib/session.svelte.js';
	import { emptyTankForm, toCreate } from '#lib/tank-form.js';

	let form = $state(emptyTankForm());
	let busy = $state(false);
	let error = $state<ApiError | null>(null);

	async function create() {
		busy = true;
		error = null;
		const r = await api<Tank>('POST', '/tanks', toCreate(form));
		busy = false;
		if (!r.ok) {
			error = r.error;
			return;
		}
		await goto(resolve('/(app)/tanks/[id]', { id: r.data.id }));
	}
</script>

<svelte:head><title>{m.tank_new_title()} · {m.app_name()}</title></svelte:head>

<h1>{m.tank_new_title()}</h1>
<TankFormView
	bind:form
	{error}
	{busy}
	submitLabel={m.tank_create_submit()}
	timeZone={session.user?.timezone}
	onsubmit={create}
	oncancel={() => goto(resolve('/(app)/tanks'))}
/>
