<script lang="ts">
	import { resolve } from '$app/paths';
	import { api, type ApiError, type KnowledgeVersion, type Page } from '#lib/api/client.js';
	import Alert from '#lib/components/Alert.svelte';
	import Field from '#lib/components/Field.svelte';
	import SelectField from '#lib/components/SelectField.svelte';
	import { errorMessage } from '#lib/errors.js';
	import { categoryLabel, claimLabel } from '#lib/knowledge.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import { session } from '#lib/session.svelte.js';
	import { loadCatalog, parameterLabel } from '#lib/water-tests.js';

	const locale = getLocale();
	let items = $state<KnowledgeVersion[]>([]);
	let loading = $state(true);
	let error = $state<ApiError | null>(null);
	let q = $state('');
	let parameter = $state('');
	let params = $state<{ value: string; label: string }[]>([]);
	const editor = $derived(session.user?.roles?.includes('administrator') ?? false);

	loadCatalog().then((r) => {
		if (r.ok) params = r.data.map((p) => ({ value: p.code, label: parameterLabel(p.code) }));
	});

	async function load() {
		loading = true;
		error = null;
		let qs = `locale=${locale}`;
		if (q.trim()) qs += `&q=${encodeURIComponent(q.trim())}`;
		if (parameter) qs += `&parameter=${encodeURIComponent(parameter)}`;
		const r = await api<Page<KnowledgeVersion>>('GET', `/knowledge?${qs}`);
		loading = false;
		if (!r.ok) return void (error = r.error);
		items = r.data.items;
	}

	$effect(() => {
		void parameter;
		load();
	});
</script>

<svelte:head><title>{m.kb_title()} · {m.app_name()}</title></svelte:head>

<h1>{m.kb_title()}</h1>
<p class="muted">{m.kb_intro()}</p>
{#if editor}
	<p><a href={resolve('/(app)/admin/knowledge')}>{m.nav_knowledge_admin()}</a></p>
{/if}

<form
	class="filters"
	onsubmit={(e) => {
		e.preventDefault();
		load();
	}}
>
	<Field label={m.kb_search()} type="search" bind:value={q} maxlength={100} />
	<SelectField
		label={m.kb_parameter()}
		name="parameter"
		bind:value={parameter}
		options={[{ value: '', label: m.kb_parameter_all() }, ...params]}
	/>
	<button type="submit">{m.kb_search()}</button>
</form>

{#if loading && items.length === 0}
	<p role="status">{m.loading()}</p>
{:else if error}
	<Alert kind="error">
		{errorMessage(error)}
		<button type="button" class="secondary" onclick={load}>{m.retry()}</button>
	</Alert>
{:else if items.length === 0}
	<p class="muted">{q || parameter ? m.kb_empty_search() : m.kb_empty()}</p>
{:else}
	<ul class="list">
		{#each items as a (a.id)}
			<li>
				<h2><a href={resolve('/(app)/knowledge/[slug]', { slug: a.slug ?? '' })}>{a.title}</a></h2>
				<p class="muted">
					{categoryLabel(a.category ?? '')} · {m.kb_claim({ label: claimLabel(a.claim_class) })}
				</p>
				<p>{a.summary}</p>
			</li>
		{/each}
	</ul>
{/if}

<style>
	.filters {
		display: flex;
		flex-wrap: wrap;
		gap: 0.75rem;
		align-items: end;
	}
	.list {
		list-style: none;
		margin: 0;
		padding: 0;
	}
	.list li {
		border-top: 1px solid var(--border, #dde);
		padding: 0.75rem 0;
	}
	h2 {
		font-size: 1.1rem;
		margin: 0 0 0.25rem;
	}
</style>
