<!--
	Searches curated and own species (GET /species) and lets the keeper pick one, keep none,
	or add a private species on the spot.
-->
<script lang="ts">
	import { api, type ApiError, type Page, type Species } from '#lib/api/client.js';
	import { errorMessage, fieldErrors } from '#lib/errors.js';
	import {
		coralTypeLabel,
		coralTypes,
		livestockCategories,
		livestockCategoryLabel,
		speciesName,
		type LivestockCategory
	} from '#lib/livestock.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import Alert from './Alert.svelte';
	import Field from './Field.svelte';
	import SelectField from './SelectField.svelte';

	type Props = {
		/** Selected species id ('' for none). */
		value: string;
		/** Name of the already chosen species when editing (the picker has no Species object). */
		currentName?: string;
		/** Called with the chosen species (null for none), e.g. to prefill name and category. */
		onpick: (s: Species | null) => void;
	};
	let { value = $bindable(), currentName = '', onpick }: Props = $props();

	const locale = getLocale();
	let q = $state('');
	let results = $state<Species[]>([]);
	let selected = $state<Species | null>(null);

	let seq = 0;
	$effect(() => {
		const query = q.trim();
		const mine = ++seq;
		const timer = setTimeout(async () => {
			const params = new URLSearchParams(query ? { limit: '20', q: query } : { limit: '20' });
			const r = await api<Page<Species>>('GET', `/species?${params.toString()}`);
			if (mine === seq) results = r.ok ? r.data.items : [];
		}, 250);
		return () => clearTimeout(timer);
	});

	function pick(s: Species | null) {
		selected = s;
		value = s?.id ?? '';
		onpick(s);
	}

	// --- Own species ------------------------------------------------------------------------
	let adding = $state(false);
	let category = $state<LivestockCategory>('fish');
	let coralType = $state('');
	let nameVi = $state('');
	let nameEn = $state('');
	let scientific = $state('');
	let busy = $state(false);
	let error = $state<ApiError | null>(null);
	const fe = $derived(fieldErrors(error));

	async function addSpecies() {
		busy = true;
		error = null;
		const r = await api<Species>('POST', '/species', {
			category,
			coral_type: category === 'coral' && coralType ? coralType : null,
			common_name_vi: nameVi.trim() || null,
			common_name_en: nameEn.trim() || null,
			scientific_name: scientific.trim() || null
		});
		busy = false;
		if (!r.ok) return void (error = r.error);
		adding = false;
		nameVi = nameEn = scientific = '';
		pick(r.data);
	}
</script>

<fieldset class="picker">
	<legend>{m.ls_species_label()}</legend>
	{#if selected}
		<p>
			<strong>{speciesName(selected, locale)}</strong>
			{#if selected.scientific_name}<em>({selected.scientific_name})</em>{/if}
			<button type="button" class="secondary" onclick={() => pick(null)}
				>{m.ls_species_clear()}</button
			>
		</p>
	{:else if value && currentName}
		<p>
			<strong>{currentName}</strong>
			<button type="button" class="secondary" onclick={() => pick(null)}
				>{m.ls_species_clear()}</button
			>
		</p>
	{:else}
		<Field
			label={m.ls_species_search()}
			name="species_q"
			type="search"
			bind:value={q}
			hint={m.ls_species_search_hint()}
		/>
		{#if results.length > 0}
			<ul class="results" aria-label={m.ls_species_results()}>
				{#each results as s (s.id)}
					<li>
						<button type="button" class="link" onclick={() => pick(s)}>
							{speciesName(s, locale)}
							{#if s.scientific_name}<em>({s.scientific_name})</em>{/if}
							· {livestockCategoryLabel(s.category)}
							{#if !s.curated}· {m.ls_species_own()}{/if}
						</button>
					</li>
				{/each}
			</ul>
		{:else if q.trim()}
			<p class="muted">{m.ls_species_none_found()}</p>
		{/if}
		<p class="hint">{m.ls_species_optional()}</p>
		{#if !adding}
			<button type="button" class="secondary" onclick={() => (adding = true)}
				>{m.ls_species_add_own()}</button
			>
		{:else}
			<div class="own">
				{#if error && error.code !== 'validation_failed'}
					<Alert kind="error">{errorMessage(error)}</Alert>
				{/if}
				<SelectField
					label={m.ls_category_label()}
					name="species_category"
					bind:value={category}
					options={livestockCategories.map((c) => ({ value: c, label: livestockCategoryLabel(c) }))}
					error={fe.category}
				/>
				{#if category === 'coral'}
					<SelectField
						label={m.ls_coral_type_label()}
						name="species_coral_type"
						bind:value={coralType}
						options={[
							{ value: '', label: m.not_set() },
							...coralTypes.map((c) => ({ value: c, label: coralTypeLabel(c) }))
						]}
						error={fe.coral_type}
					/>
				{/if}
				<Field
					label={m.ls_name_vi()}
					name="common_name_vi"
					optional
					bind:value={nameVi}
					error={fe.common_name_vi}
				/>
				<Field
					label={m.ls_name_en()}
					name="common_name_en"
					optional
					bind:value={nameEn}
					error={fe.common_name_en}
				/>
				<Field
					label={m.ls_name_scientific()}
					name="scientific_name"
					optional
					bind:value={scientific}
					error={fe.scientific_name ?? fe.common_name_vi}
				/>
				<div class="actions">
					<button type="button" disabled={busy} onclick={addSpecies}>{m.ls_species_save()}</button>
					<button type="button" class="secondary" onclick={() => (adding = false)}
						>{m.cancel()}</button
					>
				</div>
			</div>
		{/if}
	{/if}
</fieldset>

<style>
	.picker {
		border: 1px solid var(--border, #ccd);
		border-radius: 0.5rem;
		padding: 0.75rem 1rem;
	}
	.results {
		list-style: none;
		margin: 0 0 0.5rem;
		padding: 0;
	}
	.results button {
		text-align: left;
		padding: 0.35rem 0;
	}
	.own {
		margin-top: 0.75rem;
	}
</style>
