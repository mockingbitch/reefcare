<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { api, type ApiError, type LivestockEntry } from '#lib/api/client.js';
	import Alert from '#lib/components/Alert.svelte';
	import LivestockEntryFormView from '#lib/components/LivestockEntryForm.svelte';
	import { errorMessage } from '#lib/errors.js';
	import { entryToPatch, fromEntry, speciesName, type EntryForm } from '#lib/livestock.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import { session } from '#lib/session.svelte.js';

	const id = $derived(page.params.id ?? '');
	const entryId = $derived(page.params.entryId ?? '');
	const tz = session.user?.timezone ?? 'Asia/Ho_Chi_Minh';

	let original = $state<LivestockEntry | null>(null);
	let form = $state<EntryForm | null>(null);
	let loadError = $state<ApiError | null>(null);
	let busy = $state(false);
	let error = $state<ApiError | null>(null);

	const back = () => goto(resolve('/(app)/tanks/[id]/livestock/[entryId]', { id, entryId }));

	$effect(() => {
		api<LivestockEntry>('GET', `/livestock/${encodeURIComponent(entryId)}`).then((r) => {
			if (!r.ok) return void (loadError = r.error);
			original = r.data;
			form = fromEntry(r.data);
		});
	});

	async function save() {
		if (!original || !form) return;
		busy = true;
		error = null;
		const r = await api<LivestockEntry>(
			'PATCH',
			`/livestock/${original.id}`,
			entryToPatch(form, original)
		);
		busy = false;
		if (!r.ok) return void (error = r.error);
		await back();
	}
</script>

<svelte:head><title>{m.ls_edit_title()} · {m.app_name()}</title></svelte:head>

<p>
	<a href={resolve('/(app)/tanks/[id]/livestock/[entryId]', { id, entryId })}
		>← {original?.display_name ?? m.ls_title()}</a
	>
</p>
<h1>{m.ls_edit_title()}</h1>

{#if loadError}
	<Alert kind="error">{errorMessage(loadError)}</Alert>
{:else if form}
	<LivestockEntryFormView
		bind:form
		creating={false}
		speciesLabel={speciesName(original?.species, getLocale())}
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
