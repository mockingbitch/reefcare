<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import {
		api,
		type ApiError,
		type KnowledgeArticle,
		type KnowledgeSource,
		type Page
	} from '#lib/api/client.js';
	import Alert from '#lib/components/Alert.svelte';
	import Field from '#lib/components/Field.svelte';
	import SelectField from '#lib/components/SelectField.svelte';
	import { errorMessage, fieldErrors } from '#lib/errors.js';
	import { categories, categoryLabel, statusLabel } from '#lib/knowledge.js';
	import { m } from '#lib/paraglide/messages.js';

	let articles = $state<KnowledgeArticle[]>([]);
	let sources = $state<KnowledgeSource[]>([]);
	let loading = $state(true);
	let error = $state<ApiError | null>(null);

	let slug = $state('');
	let category = $state<string>('water_chemistry');
	let articleErrors = $state<Record<string, string>>({});
	let articleError = $state<ApiError | null>(null);

	let src = $state({ title: '', publisher: '', url: '', published_on: '', accessed_on: '' });
	let sourceErrors = $state<Record<string, string>>({});
	let sourceError = $state<ApiError | null>(null);

	async function load() {
		loading = true;
		error = null;
		const [a, s] = await Promise.all([
			api<Page<KnowledgeArticle>>('GET', '/admin/knowledge/articles'),
			api<Page<KnowledgeSource>>('GET', '/admin/knowledge/sources')
		]);
		loading = false;
		if (!a.ok) return void (error = a.error);
		if (!s.ok) return void (error = s.error);
		articles = a.data.items;
		sources = s.data.items;
	}
	load();

	async function createArticle(e: SubmitEvent) {
		e.preventDefault();
		articleErrors = {};
		articleError = null;
		const r = await api<KnowledgeArticle>('POST', '/admin/knowledge/articles', {
			slug: slug.trim(),
			category
		});
		if (!r.ok) {
			articleErrors = fieldErrors(r.error);
			if (!Object.keys(articleErrors).length) articleError = r.error;
			return;
		}
		await goto(resolve('/(app)/admin/knowledge/[articleId]', { articleId: r.data.id }));
	}

	async function createSource(e: SubmitEvent) {
		e.preventDefault();
		sourceErrors = {};
		sourceError = null;
		const opt = (s: string) => (s.trim() ? s.trim() : null);
		const r = await api<KnowledgeSource>('POST', '/admin/knowledge/sources', {
			title: src.title,
			publisher: opt(src.publisher),
			url: opt(src.url),
			published_on: opt(src.published_on),
			accessed_on: opt(src.accessed_on)
		});
		if (!r.ok) {
			sourceErrors = fieldErrors(r.error);
			if (!Object.keys(sourceErrors).length) sourceError = r.error;
			return;
		}
		sources = [...sources, r.data];
		src = { title: '', publisher: '', url: '', published_on: '', accessed_on: '' };
	}
</script>

<svelte:head><title>{m.kb_admin_title()} · {m.app_name()}</title></svelte:head>

<h1>{m.kb_admin_title()}</h1>
<p class="muted">{m.kb_admin_intro()}</p>

{#if loading}
	<p role="status">{m.loading()}</p>
{:else if error}
	<Alert kind="error">
		{error.code === 'forbidden' ? m.kb_admin_forbidden() : errorMessage(error)}
		{#if error.code !== 'forbidden'}
			<button type="button" class="secondary" onclick={load}>{m.retry()}</button>
		{/if}
	</Alert>
{:else}
	<section>
		<h2>{m.kb_articles()}</h2>
		{#if articles.length === 0}
			<p class="muted">{m.kb_no_articles()}</p>
		{:else}
			<ul class="list">
				{#each articles as a (a.id)}
					<li>
						<a href={resolve('/(app)/admin/knowledge/[articleId]', { articleId: a.id })}>{a.slug}</a
						>
						<span class="muted">· {categoryLabel(a.category)}</span>
						{#if a.retired_at}<span class="badge">{m.kb_retired()}</span>{/if}
						<ul class="versions">
							{#each a.latest as v (v.id)}
								<li>
									{v.locale.toUpperCase()} v{v.version_no}: {v.title}
									<span class="badge">{statusLabel(v.status)}</span>
								</li>
							{/each}
						</ul>
					</li>
				{/each}
			</ul>
		{/if}

		<form onsubmit={createArticle} class="card">
			<h3>{m.kb_new_article()}</h3>
			{#if articleError}<Alert kind="error">{errorMessage(articleError)}</Alert>{/if}
			<Field
				label={m.kb_slug()}
				hint={m.kb_slug_hint()}
				bind:value={slug}
				error={articleErrors.slug}
				required
			/>
			<SelectField
				label={m.kb_category()}
				name="category"
				bind:value={category}
				error={articleErrors.category}
				options={categories.map((c) => ({ value: c, label: categoryLabel(c) }))}
			/>
			<button type="submit">{m.kb_create()}</button>
		</form>
	</section>

	<section>
		<h2>{m.kb_sources()}</h2>
		<p class="muted">{m.kb_source_hint()}</p>
		{#if sources.length === 0}
			<p class="muted">{m.kb_no_sources()}</p>
		{:else}
			<ul>
				{#each sources as s (s.id)}
					<li>
						{s.title}{#if s.publisher}
							· {s.publisher}{/if}{#if s.url}
							· <span class="url">{s.url}</span>{/if}
					</li>
				{/each}
			</ul>
		{/if}
		<form onsubmit={createSource} class="card">
			<h3>{m.kb_new_source()}</h3>
			{#if sourceError}<Alert kind="error">{errorMessage(sourceError)}</Alert>{/if}
			<Field
				label={m.kb_source_title()}
				bind:value={src.title}
				error={sourceErrors.title}
				required
			/>
			<Field
				label={m.kb_source_publisher()}
				bind:value={src.publisher}
				error={sourceErrors.publisher}
				optional
			/>
			<Field
				label={m.kb_source_url()}
				type="url"
				bind:value={src.url}
				error={sourceErrors.url}
				optional
			/>
			<Field
				label={m.kb_source_published_on()}
				bind:value={src.published_on}
				error={sourceErrors.published_on}
				optional
			/>
			<Field
				label={m.kb_source_accessed_on()}
				bind:value={src.accessed_on}
				error={sourceErrors.accessed_on}
				optional
			/>
			<button type="submit">{m.kb_add()}</button>
		</form>
	</section>
{/if}

<style>
	.list,
	.versions {
		list-style: none;
		padding: 0;
	}
	.list > li {
		border-top: 1px solid var(--border, #dde);
		padding: 0.5rem 0;
	}
	.versions {
		margin: 0.25rem 0 0 1rem;
		font-size: 0.95rem;
	}
	.card {
		margin-top: 1rem;
		max-width: 36rem;
	}
	.url {
		word-break: break-all;
	}
</style>
