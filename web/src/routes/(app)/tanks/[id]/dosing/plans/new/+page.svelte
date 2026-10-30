<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import {
		api,
		type ApiError,
		type DosingPlan,
		type DosingProduct,
		type Page
	} from '#lib/api/client.js';
	import Alert from '#lib/components/Alert.svelte';
	import PlanFormView from '#lib/components/PlanForm.svelte';
	import { emptyPlanForm, planToCreate } from '#lib/dosing.js';
	import { errorMessage } from '#lib/errors.js';
	import { today } from '#lib/format.js';
	import { m } from '#lib/paraglide/messages.js';
	import { session } from '#lib/session.svelte.js';

	const id = $derived(page.params.id ?? '');
	const tz = session.user?.timezone ?? 'Asia/Ho_Chi_Minh';

	let form = $state(emptyPlanForm(today(tz), tz));
	let products = $state<DosingProduct[]>([]);
	let loadError = $state<ApiError | null>(null);
	let busy = $state(false);
	let error = $state<ApiError | null>(null);

	const back = () => goto(resolve('/(app)/tanks/[id]/dosing', { id }));

	$effect(() => {
		api<Page<DosingProduct>>('GET', '/dosing-products').then((r) => {
			if (r.ok) products = r.data.items;
			else loadError = r.error;
		});
	});

	async function create() {
		busy = true;
		error = null;
		const r = await api<DosingPlan>(
			'POST',
			`/tanks/${encodeURIComponent(id)}/dosing-plans`,
			planToCreate(form)
		);
		busy = false;
		if (!r.ok) {
			error = r.error;
			return;
		}
		await back();
	}
</script>

<svelte:head><title>{m.dose_plan_new()} · {m.app_name()}</title></svelte:head>

<p><a href={resolve('/(app)/tanks/[id]/dosing', { id })}>← {m.dose_title()}</a></p>
<h1>{m.dose_plan_new()}</h1>

{#if loadError}
	<Alert kind="error">{errorMessage(loadError)}</Alert>
{:else}
	{#if products.length === 0}
		<Alert kind="info">{m.dose_no_products()}</Alert>
	{/if}
	<PlanFormView
		bind:form
		{products}
		{error}
		{busy}
		submitLabel={m.dose_plan_create_submit()}
		onsubmit={create}
		oncancel={back}
	/>
{/if}
