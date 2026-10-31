<script lang="ts">
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { api, type ApiError, type KnowledgeVersion } from '#lib/api/client.js';
	import Alert from '#lib/components/Alert.svelte';
	import { errorMessage } from '#lib/errors.js';
	import { formatDate } from '#lib/format.js';
	import { categoryLabel, claimLabel, safeHref } from '#lib/knowledge.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import { parameterLabel } from '#lib/water-tests.js';

	const locale = getLocale();
	let v = $state<KnowledgeVersion | null>(null);
	let loading = $state(true);
	let error = $state<ApiError | null>(null);

	async function load() {
		loading = true;
		error = null;
		const r = await api<KnowledgeVersion>(
			'GET',
			`/knowledge/${encodeURIComponent(page.params.slug ?? '')}?locale=${locale}`
		);
		loading = false;
		if (!r.ok) return void (error = r.error);
		v = r.data;
	}
	$effect(() => {
		void page.params.slug;
		load();
	});

	const lists = $derived(
		v?.structured
			? [
					{ title: m.kb_safety_warnings(), items: v.structured.safety_warnings ?? [], warn: true },
					{ title: m.kb_verifications(), items: v.structured.verifications ?? [], warn: false },
					{
						title: m.kb_recommendations(),
						items: (v.structured.recommendations ?? []).map((r) => ({
							key: r.key,
							text: `${r.text} (${claimLabel(r.claim_class)})`
						})),
						warn: false
					},
					{
						title: m.kb_contraindications(),
						items: v.structured.contraindications ?? [],
						warn: false
					}
				].filter((l) => l.items.length > 0)
			: []
	);
</script>

<svelte:head><title>{v?.title ?? m.kb_title()} · {m.app_name()}</title></svelte:head>

<p><a href={resolve('/(app)/knowledge')}>← {m.kb_back()}</a></p>

{#if loading && !v}
	<p role="status">{m.loading()}</p>
{:else if error}
	<Alert kind="error">
		{errorMessage(error)}
		<button type="button" class="secondary" onclick={load}>{m.retry()}</button>
	</Alert>
{:else if v}
	<article lang={v.locale}>
		<h1>{v.title}</h1>
		<p class="muted">
			{categoryLabel(v.category ?? '')} · {m.kb_claim({ label: claimLabel(v.claim_class) })} ·
			{m.kb_meta({
				version: String(v.version_no),
				date: v.published_at ? formatDate(v.published_at, locale) : ''
			})}
		</p>
		{#if v.locale !== locale}
			<Alert kind="info">{m.kb_other_locale()}</Alert>
		{/if}
		{#if v.claim_class === 'community' || v.claim_class === 'unverified'}
			<Alert kind="warning">{m.kb_unverified_note()}</Alert>
		{/if}
		<p class="summary">{v.summary}</p>
		{#if v.parameters.length}
			<p class="muted">{v.parameters.map(parameterLabel).join(', ')}</p>
		{/if}
		<!-- Untrusted text: shown as text (never rendered as HTML). -->
		<div class="body">{v.body_md}</div>

		{#each lists as l (l.title)}
			<section>
				<h2>
					{#if l.warn}⚠
					{/if}{l.title}
				</h2>
				<ul>
					{#each l.items as it (it.key)}<li>{it.text}</li>{/each}
				</ul>
			</section>
		{/each}

		<section>
			<h2>{m.kb_sources()}</h2>
			{#if !v.sources?.length}
				<p class="muted">{m.kb_no_sources()}</p>
			{:else}
				<ul>
					{#each v.sources as s (s.id)}
						<li>
							{#if safeHref(s.url)}
								<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -- external https source, not an app route -->
								<a href={safeHref(s.url)} rel="noopener noreferrer nofollow" target="_blank"
									>{s.title}</a
								>
							{:else}{s.title}{/if}
							{#if s.publisher}· {s.publisher}{/if}
							{#if s.published_on}· {formatDate(s.published_on, locale)}{/if}
							{#if s.note}— {s.note}{/if}
						</li>
					{/each}
				</ul>
			{/if}
		</section>
	</article>
{/if}

<style>
	.summary {
		font-weight: 600;
	}
	.body {
		white-space: pre-wrap;
		line-height: 1.6;
	}
	h2 {
		font-size: 1.1rem;
		margin-top: 1.5rem;
	}
</style>
