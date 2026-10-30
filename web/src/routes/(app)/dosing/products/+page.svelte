<!-- The curated product catalog and the user's own products. -->
<script lang="ts">
	import {
		api,
		type ApiError,
		type DoseUnit,
		type DosingForm,
		type DosingProduct,
		type Page
	} from '#lib/api/client.js';
	import Alert from '#lib/components/Alert.svelte';
	import Field from '#lib/components/Field.svelte';
	import SelectField from '#lib/components/SelectField.svelte';
	import { unitLabel, unitsForForm } from '#lib/dosing.js';
	import { errorMessage, fieldErrors } from '#lib/errors.js';
	import { m } from '#lib/paraglide/messages.js';

	let products = $state<DosingProduct[]>([]);
	let loading = $state(true);
	let loadError = $state<ApiError | null>(null);

	let name = $state('');
	let brand = $state('');
	let form = $state<DosingForm>('liquid');
	let unit = $state<DoseUnit>('ml');
	let notes = $state('');
	let busy = $state(false);
	let error = $state<ApiError | null>(null);
	let created = $state('');
	const fe = $derived(fieldErrors(error));
	const units = $derived(unitsForForm(form));

	$effect(() => {
		if (!units.includes(unit)) unit = units[0];
	});

	async function load() {
		loading = true;
		const r = await api<Page<DosingProduct>>('GET', '/dosing-products');
		loading = false;
		if (r.ok) products = r.data.items;
		else loadError = r.error;
	}

	$effect(() => {
		load();
	});

	async function create() {
		busy = true;
		error = null;
		created = '';
		const r = await api<DosingProduct>('POST', '/dosing-products', {
			name: name.trim(),
			brand: brand.trim() || null,
			form,
			default_unit: unit,
			notes: notes.trim() || null
		});
		busy = false;
		if (!r.ok) {
			error = r.error;
			return;
		}
		created = r.data.name;
		name = brand = notes = '';
		await load();
	}

	const formLabels: Record<DosingForm, () => string> = {
		liquid: m.dose_form_liquid,
		powder: m.dose_form_powder,
		tablet: m.dose_form_tablet,
		other: m.dose_form_other
	};
</script>

<svelte:head><title>{m.dose_products_title()} · {m.app_name()}</title></svelte:head>

<h1>{m.dose_products_title()}</h1>
<p class="muted">{m.dose_products_intro()}</p>

<section class="card" aria-labelledby="list">
	<h2 id="list">{m.dose_products_list()}</h2>
	{#if loading}
		<p role="status">{m.loading()}</p>
	{:else if loadError}
		<Alert kind="error">{errorMessage(loadError)}</Alert>
	{:else if products.length === 0}
		<p class="muted">{m.dose_no_products()}</p>
	{:else}
		<ul>
			{#each products as p (p.id)}
				<li>
					<strong>{p.name}</strong>{#if p.brand}
						({p.brand}){/if} · {formLabels[p.form]()} · {unitLabel(p.default_unit)}
					{#if p.curated}<span class="badge">{m.dose_product_curated()}</span>{/if}
				</li>
			{/each}
		</ul>
	{/if}
</section>

<section class="card" aria-labelledby="new">
	<h2 id="new">{m.dose_product_new()}</h2>
	{#if created}<Alert kind="success">{m.dose_product_created({ name: created })}</Alert>{/if}
	{#if error && error.code !== 'validation_failed'}
		<Alert kind="error">{errorMessage(error)}</Alert>
	{/if}
	<form
		novalidate
		onsubmit={(e) => {
			e.preventDefault();
			create();
		}}
	>
		<Field label={m.dose_product_name()} name="name" required bind:value={name} error={fe.name} />
		<Field
			label={m.dose_product_brand()}
			name="brand"
			optional
			bind:value={brand}
			error={fe.brand}
		/>
		<SelectField
			label={m.dose_product_form()}
			name="form"
			bind:value={form}
			options={(['liquid', 'powder', 'tablet', 'other'] as const).map((f) => ({
				value: f,
				label: formLabels[f]()
			}))}
			error={fe.form}
		/>
		<SelectField
			label={m.dose_product_default_unit()}
			name="default_unit"
			bind:value={unit}
			options={units.map((u) => ({ value: u, label: unitLabel(u) }))}
			error={fe.default_unit}
		/>
		<Field
			label={m.wc_notes_label()}
			name="notes"
			optional
			multiline
			bind:value={notes}
			error={fe.notes}
		/>
		<p class="hint">{m.dose_product_no_concentration()}</p>
		<div class="actions">
			<button type="submit" disabled={busy}>{busy ? m.saving() : m.dose_product_create()}</button>
		</div>
	</form>
</section>
