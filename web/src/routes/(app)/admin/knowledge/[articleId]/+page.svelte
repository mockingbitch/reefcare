<script lang="ts">
	import { goto } from '$app/navigation';
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
	import ConfirmDialog from '#lib/components/ConfirmDialog.svelte';
	import KnowledgeVersionForm from '#lib/components/KnowledgeVersionForm.svelte';
	import SelectField from '#lib/components/SelectField.svelte';
	import { errorMessage } from '#lib/errors.js';
	import { formatDateTime } from '#lib/format.js';
	import { claimLabel, statusLabel } from '#lib/knowledge.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import { session } from '#lib/session.svelte.js';

	const id = $derived(page.params.articleId ?? '');
	const locale = getLocale();
	const tz = $derived(session.user?.timezone ?? 'Asia/Ho_Chi_Minh');

	let versions = $state<KnowledgeVersion[]>([]);
	let sources = $state<KnowledgeSource[]>([]);
	let loading = $state(true);
	let error = $state<ApiError | null>(null);
	let draftLocale = $state('vi');
	let template = $state<KnowledgeVersion | null>(null);
	let creating = $state(false);
	let confirmRetire = $state(false);

	async function load() {
		loading = true;
		error = null;
		const [v, s] = await Promise.all([
			api<Page<KnowledgeVersion>>('GET', `/admin/knowledge/articles/${id}/versions`),
			api<Page<KnowledgeSource>>('GET', '/admin/knowledge/sources')
		]);
		loading = false;
		if (!v.ok) return void (error = v.error);
		if (!s.ok) return void (error = s.error);
		versions = v.data.items;
		sources = s.data.items;
	}
	$effect(() => {
		void id;
		load();
	});

	// A new draft starts from the newest version in that locale, if any.
	async function startDraft() {
		const latest = versions.find((v) => v.locale === draftLocale);
		template = null;
		if (latest) {
			const r = await api<KnowledgeVersion>('GET', `/admin/knowledge/versions/${latest.id}`);
			if (!r.ok) return void (error = r.error);
			template = r.data;
		}
		creating = true;
	}

	async function create(input: KnowledgeVersionInput): Promise<ApiError | null> {
		const r = await api<KnowledgeVersion>('POST', `/admin/knowledge/articles/${id}/versions`, {
			...input,
			locale: draftLocale
		});
		if (!r.ok) return r.error;
		await goto(resolve('/(app)/admin/knowledge/versions/[id]', { id: r.data.id }));
		return null;
	}

	async function retire() {
		confirmRetire = false;
		const r = await api('POST', `/admin/knowledge/articles/${id}/retire`);
		if (!r.ok) return void (error = r.error);
		await load();
	}
</script>

<svelte:head><title>{m.kb_versions()} · {m.app_name()}</title></svelte:head>

<p><a href={resolve('/(app)/admin/knowledge')}>← {m.kb_admin_title()}</a></p>
<h1>{m.kb_versions()}</h1>

{#if loading}
	<p role="status">{m.loading()}</p>
{:else if error}
	<Alert kind="error">
		{error.code === 'forbidden' ? m.kb_admin_forbidden() : errorMessage(error)}
	</Alert>
{:else}
	{#if versions.length === 0}
		<p class="muted">{m.kb_no_versions()}</p>
	{:else}
		<ul class="list">
			{#each versions as v (v.id)}
				<li>
					<a href={resolve('/(app)/admin/knowledge/versions/[id]', { id: v.id })}
						>{v.locale.toUpperCase()} v{v.version_no}: {v.title}</a
					>
					<span class="badge">{statusLabel(v.status)}</span>
					<span class="muted"
						>{claimLabel(v.claim_class)}
						{#if v.updated_at}· {formatDateTime(v.updated_at, locale, tz)}{/if}</span
					>
				</li>
			{/each}
		</ul>
	{/if}

	{#if !creating}
		<div class="actions">
			<SelectField
				label={m.kb_locale()}
				name="locale"
				bind:value={draftLocale}
				options={[
					{ value: 'vi', label: m.kb_locale_vi() },
					{ value: 'en', label: m.kb_locale_en() }
				]}
			/>
			<button type="button" onclick={startDraft}>{m.kb_new_draft()}</button>
			<button type="button" class="secondary" onclick={() => (confirmRetire = true)}
				>{m.kb_retire_article()}</button
			>
		</div>
	{:else}
		<h2>{m.kb_new_draft()} ({draftLocale.toUpperCase()})</h2>
		<KnowledgeVersionForm initial={template} {sources} submitLabel={m.kb_save()} save={create} />
	{/if}
{/if}

<ConfirmDialog
	bind:open={confirmRetire}
	title={m.kb_retire_article()}
	confirmLabel={m.kb_retire_article()}
	danger
	onconfirm={retire}
>
	<p>{m.kb_retire_confirm()}</p>
</ConfirmDialog>

<style>
	.list {
		list-style: none;
		padding: 0;
	}
	.list li {
		border-top: 1px solid var(--border, #dde);
		padding: 0.5rem 0;
	}
	.actions {
		display: flex;
		flex-wrap: wrap;
		gap: 0.75rem;
		align-items: end;
		margin-top: 1rem;
	}
</style>
