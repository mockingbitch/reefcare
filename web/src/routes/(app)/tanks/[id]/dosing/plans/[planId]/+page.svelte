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
	import { fromPlan, planToPatch, type PlanForm } from '#lib/dosing.js';
	import { errorMessage } from '#lib/errors.js';
	import { m } from '#lib/paraglide/messages.js';

	const id = $derived(page.params.id ?? '');
	const planId = $derived(page.params.planId ?? '');

	let original = $state<DosingPlan | null>(null);
	let form = $state<PlanForm | null>(null);
	let products = $state<DosingProduct[]>([]);
	let loading = $state(true);
	let loadError = $state<ApiError | null>(null);
	let busy = $state(false);
	let error = $state<ApiError | null>(null);

	const back = () => goto(resolve('/(app)/tanks/[id]/dosing', { id }));

	async function load() {
		loading = true;
		// No single-plan GET: a tank has few plans, so find it in the list.
		const [p, prods] = await Promise.all([
			api<Page<DosingPlan>>('GET', `/tanks/${encodeURIComponent(id)}/dosing-plans`),
			api<Page<DosingProduct>>('GET', '/dosing-products')
		]);
		loading = false;
		if (!p.ok) {
			loadError = p.error;
			return;
		}
		original = p.data.items.find((x) => x.id === planId) ?? null;
		form = original ? fromPlan(original) : null;
		// The plan's product is always listed, even if it is no longer in the catalog.
		const listed = prods.ok ? prods.data.items : [];
		products =
			original && !listed.some((x) => x.id === original!.product.id)
				? [
						...listed,
						{
							id: original.product.id,
							name: original.product.name,
							brand: original.product.brand ?? null,
							form: original.product.form ?? 'other',
							default_unit: original.amount_unit,
							curated: false,
							notes: null,
							created_at: original.created_at
						}
					]
				: listed;
	}

	$effect(() => {
		void planId;
		load();
	});

	async function save() {
		if (!original || !form) return;
		busy = true;
		error = null;
		const r = await api<DosingPlan>(
			'PATCH',
			`/dosing-plans/${original.id}`,
			planToPatch(form, original)
		);
		busy = false;
		if (!r.ok) {
			error = r.error;
			return;
		}
		await back();
	}
</script>

<svelte:head><title>{m.dose_plan_edit_title()} · {m.app_name()}</title></svelte:head>

<p><a href={resolve('/(app)/tanks/[id]/dosing', { id })}>← {m.dose_title()}</a></p>
<h1>{m.dose_plan_edit_title()}</h1>

{#if loading}
	<p role="status">{m.loading()}</p>
{:else if loadError}
	<Alert kind="error">{errorMessage(loadError)}</Alert>
{:else if !form}
	<p>{m.not_found_body()}</p>
{:else}
	<PlanFormView
		bind:form
		{products}
		productLocked
		{error}
		{busy}
		submitLabel={m.save()}
		onsubmit={save}
		oncancel={back}
	/>
{/if}
