<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { api, type ApiError, type LivestockEntry } from '#lib/api/client.js';
	import LivestockEntryFormView from '#lib/components/LivestockEntryForm.svelte';
	import { emptyEntryForm, entryToCreate } from '#lib/livestock.js';
	import { m } from '#lib/paraglide/messages.js';
	import { session } from '#lib/session.svelte.js';
	import { fromLocalInput, toLocalInput } from '#lib/water-changes.js';

	const id = $derived(page.params.id ?? '');
	const tz = session.user?.timezone ?? 'Asia/Ho_Chi_Minh';

	let form = $state(emptyEntryForm());
	let addedAt = $state(toLocalInput(new Date().toISOString(), tz));
	let busy = $state(false);
	let error = $state<ApiError | null>(null);
	// One key per form: a retried submit adds the animals once.
	const idempotencyKey = crypto.randomUUID();

	async function create() {
		busy = true;
		error = null;
		const r = await api<LivestockEntry>(
			'POST',
			`/tanks/${encodeURIComponent(id)}/livestock`,
			entryToCreate(form, fromLocalInput(addedAt, tz)),
			fetch,
			{ 'Idempotency-Key': idempotencyKey }
		);
		busy = false;
		if (!r.ok) return void (error = r.error);
		await goto(resolve('/(app)/tanks/[id]/livestock/[entryId]', { id, entryId: r.data.id }));
	}
</script>

<svelte:head><title>{m.ls_add()} · {m.app_name()}</title></svelte:head>

<p><a href={resolve('/(app)/tanks/[id]/livestock', { id })}>← {m.ls_title()}</a></p>
<h1>{m.ls_add()}</h1>
<LivestockEntryFormView
	bind:form
	bind:addedAt
	creating
	timeZone={tz}
	{error}
	{busy}
	submitLabel={m.ls_add()}
	onsubmit={create}
	oncancel={() => goto(resolve('/(app)/tanks/[id]/livestock', { id }))}
/>
