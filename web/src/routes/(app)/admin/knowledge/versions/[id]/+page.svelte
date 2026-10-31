<script lang="ts">
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import {
		api,
		type ApiError,
		type KnowledgeSource,
		type KnowledgeVersion,
		type KnowledgeVersionInput,
		type Page
	} from '#lib/api/client.js';
	import Alert from '#lib/components/Alert.svelte';
	import Field from '#lib/components/Field.svelte';
	import KnowledgeVersionForm from '#lib/components/KnowledgeVersionForm.svelte';
	import { errorMessage } from '#lib/errors.js';
	import { formatDateTime } from '#lib/format.js';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import { claimLabel, statusLabel } from '#lib/knowledge.js';
	import { m } from '#lib/paraglide/messages.js';
	import { session } from '#lib/session.svelte.js';
	import { parameterLabel } from '#lib/water-tests.js';

	const id = $derived(page.params.id ?? '');
	let v = $state<KnowledgeVersion | null>(null);
	let sources = $state<KnowledgeSource[]>([]);
	let loading = $state(true);
	let error = $state<ApiError | null>(null);
	let actionError = $state<ApiError | null>(null);
	let note = $state('');
	let noteError = $state('');
	let saved = $state(false);
	let busy = $state(false);
	let formKey = $state(0);
	const tz = $derived(session.user?.timezone ?? 'Asia/Ho_Chi_Minh');
	const isAuthor = $derived(!!v?.author_id && v.author_id === session.user?.id);

	async function load() {
		loading = true;
		error = null;
		const [r, s] = await Promise.all([
			api<KnowledgeVersion>('GET', `/admin/knowledge/versions/${id}`),
			api<Page<KnowledgeSource>>('GET', '/admin/knowledge/sources')
		]);
		loading = false;
		if (!r.ok) return void (error = r.error);
		if (!s.ok) return void (error = s.error);
		v = r.data;
		sources = s.data.items;
		formKey++;
	}
	$effect(() => {
		void id;
		load();
	});

	async function save(input: KnowledgeVersionInput): Promise<ApiError | null> {
		saved = false;
		const r = await api<KnowledgeVersion>('PATCH', `/admin/knowledge/versions/${id}`, {
			...input,
			revision: v?.revision
		});
		if (!r.ok) return r.error;
		v = r.data;
		saved = true;
		return null;
	}

	async function act(action: 'submit' | 'publish' | 'reject') {
		actionError = null;
		noteError = '';
		if (action === 'reject' && !note.trim()) return void (noteError = m.kb_review_note_required());
		busy = true;
		const body = action === 'submit' ? undefined : note.trim() ? { note: note.trim() } : undefined;
		const r = await api<KnowledgeVersion>(
			'POST',
			`/admin/knowledge/versions/${id}/${action}`,
			body
		);
		busy = false;
		if (!r.ok) return void (actionError = r.error);
		v = r.data;
		note = '';
		formKey++;
	}
</script>

<svelte:head><title>{v?.title ?? m.kb_versions()} · {m.app_name()}</title></svelte:head>

{#if v}
	<p>
		<a href={resolve('/(app)/admin/knowledge/[articleId]', { articleId: v.article_id ?? '' })}
			>← {m.kb_versions()}</a
		>
	</p>
{/if}

{#if loading && !v}
	<p role="status">{m.loading()}</p>
{:else if error}
	<Alert kind="error">
		{error.code === 'forbidden' ? m.kb_admin_forbidden() : errorMessage(error)}
	</Alert>
{:else if v}
	<h1>{v.title}</h1>
	<p>
		<span class="badge">{statusLabel(v.status)}</span>
		<span class="muted"
			>{v.slug} · {v.locale.toUpperCase()} v{v.version_no} · {claimLabel(v.claim_class)}</span
		>
	</p>
	{#if v.review_note}
		<Alert kind="info">{m.kb_previous_note({ note: v.review_note })}</Alert>
	{/if}
	{#if actionError}<Alert kind="error">{errorMessage(actionError)}</Alert>{/if}
	{#if v.reviews?.length}
		<details>
			<summary>{m.kb_reviews()}</summary>
			<ul>
				{#each v.reviews as r, i (i)}
					<li>
						{r.decision === 'rejected' ? m.kb_review_rejected() : m.kb_review_published()} ·
						<time datetime={r.at}>{formatDateTime(r.at, getLocale(), tz)}</time>
						{#if r.note}<div class="body">{r.note}</div>{/if}
					</li>
				{/each}
			</ul>
		</details>
	{/if}

	{#if v.status === 'draft' && !isAuthor}
		<Alert kind="info">{m.err_not_author()}</Alert>
		<div class="body">{v.body_md}</div>
	{:else if v.status === 'draft'}
		{#if saved}<Alert kind="success">{m.kb_saved()}</Alert>{/if}
		{#key formKey}
			<KnowledgeVersionForm initial={v} {sources} submitLabel={m.kb_save()} {save} />
		{/key}
		<div class="actions">
			<button type="button" disabled={busy} onclick={() => act('submit')}>{m.kb_submit()}</button>
		</div>
	{:else}
		<Alert kind="info">{m.kb_frozen()}</Alert>
		<p class="summary">{v.summary}</p>
		{#if v.parameters.length}<p class="muted">{v.parameters.map(parameterLabel).join(', ')}</p>{/if}
		<div class="body">{v.body_md}</div>
		<h2>{m.kb_sources()}</h2>
		{#if !v.sources?.length}
			<p class="muted">{m.kb_no_sources()}</p>
		{:else}
			<ul>
				{#each v.sources as s (s.id)}<li>
						{s.title}{#if s.publisher}
							· {s.publisher}{/if}
					</li>{/each}
			</ul>
		{/if}

		{#if v.status === 'in_review'}
			<h2>{m.kb_review_note()}</h2>
			{#if isAuthor}<Alert kind="info">{m.err_own_review()}</Alert>{/if}
			<Field label={m.kb_review_note()} bind:value={note} error={noteError} multiline optional />
			<div class="actions">
				<button type="button" disabled={busy || isAuthor} onclick={() => act('publish')}
					>{m.kb_publish()}</button
				>
				<button
					type="button"
					class="secondary"
					disabled={busy || isAuthor}
					onclick={() => act('reject')}>{m.kb_reject()}</button
				>
			</div>
		{/if}
	{/if}
{/if}

<style>
	.summary {
		font-weight: 600;
	}
	.body {
		white-space: pre-wrap;
		line-height: 1.6;
	}
	.actions {
		display: flex;
		gap: 0.75rem;
		margin-top: 1rem;
	}
</style>
