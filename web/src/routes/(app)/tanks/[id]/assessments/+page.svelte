<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { api, type ApiError, type Assessment, type Page, type Tank } from '#lib/api/client.js';
	import Alert from '#lib/components/Alert.svelte';
	import { confidenceLabel, severityIcon, severityLabel, triggerLabel } from '#lib/assessments.js';
	import { errorMessage } from '#lib/errors.js';
	import { formatDateTime } from '#lib/format.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import { session } from '#lib/session.svelte.js';

	const id = $derived(page.params.id ?? '');
	const locale = getLocale();
	const tz = $derived(session.user?.timezone ?? 'Asia/Ho_Chi_Minh');

	let tank = $state<Tank | null>(null);
	let items = $state<Assessment[]>([]);
	let cursor = $state<string | null>(null);
	let loading = $state(true);
	let error = $state<ApiError | null>(null);
	let runError = $state<ApiError | null>(null);
	let running = $state(false);
	const canRun = $derived(tank?.access === 'owner' && tank.status !== 'archived');

	async function load() {
		loading = true;
		error = null;
		const [t, a] = await Promise.all([
			api<Tank>('GET', `/tanks/${id}`),
			api<Page<Assessment>>('GET', `/tanks/${id}/assessments?limit=20`)
		]);
		loading = false;
		if (!t.ok) return void (error = t.error);
		if (!a.ok) return void (error = a.error);
		tank = t.data;
		items = a.data.items;
		cursor = a.data.next_cursor;
	}
	$effect(() => {
		void id;
		load();
	});

	async function more() {
		if (!cursor) return;
		const r = await api<Page<Assessment>>(
			'GET',
			`/tanks/${id}/assessments?limit=20&cursor=${encodeURIComponent(cursor)}`
		);
		if (!r.ok) return void (error = r.error);
		items = [...items, ...r.data.items];
		cursor = r.data.next_cursor;
	}

	async function run() {
		running = true;
		runError = null;
		const r = await api<Assessment>('POST', `/tanks/${id}/assessments`);
		running = false;
		if (!r.ok) return void (runError = r.error);
		await goto(
			resolve('/(app)/tanks/[id]/assessments/[assessmentId]', { id, assessmentId: r.data.id })
		);
	}
</script>

<svelte:head><title>{m.ia_title()} · {m.app_name()}</title></svelte:head>

<p><a href={resolve('/(app)/tanks/[id]', { id })}>← {tank?.name ?? m.nav_tanks()}</a></p>
<h1>{m.ia_title()}</h1>
<p class="muted">{m.ia_intro()}</p>

{#if canRun}
	<div class="actions">
		<button type="button" onclick={run} disabled={running}>
			{running ? m.ia_running() : m.ia_run()}
		</button>
	</div>
	{#if runError}<Alert kind="error">{errorMessage(runError)}</Alert>{/if}
{/if}

{#if loading && items.length === 0}
	<p role="status">{m.loading()}</p>
{:else if error}
	<Alert kind="error">
		{errorMessage(error)}
		<button type="button" class="secondary" onclick={load}>{m.retry()}</button>
	</Alert>
{:else if items.length === 0}
	<p class="muted">{m.ia_empty()}</p>
{:else}
	<ul class="list">
		{#each items as a (a.id)}
			<li>
				<a
					href={resolve('/(app)/tanks/[id]/assessments/[assessmentId]', { id, assessmentId: a.id })}
				>
					<time datetime={a.created_at}>{formatDateTime(a.created_at, locale, tz)}</time>
				</a>
				<span class="muted">· {triggerLabel(a.trigger)}</span>
				{#if a.status === 'retracted'}<span class="badge">{m.ia_retracted()}</span>{/if}
				<div>
					{#if a.max_severity}
						<span class="badge sev-{a.max_severity}"
							><span aria-hidden="true">{severityIcon(a.max_severity)}</span>
							{severityLabel(a.max_severity)}</span
						>
						{m.ia_findings_count({ count: String(a.finding_count) })}
					{:else}
						{m.ia_no_findings()}
					{/if}
					<span class="muted">· {confidenceLabel(a.confidence_label)}</span>
				</div>
			</li>
		{/each}
	</ul>
	{#if cursor}
		<button type="button" class="secondary" onclick={more}>{m.load_more()}</button>
	{/if}
{/if}

<style>
	.list {
		list-style: none;
		margin: 0;
		padding: 0;
	}
	.list li {
		border-top: 1px solid var(--border, #dde);
		padding: 0.6rem 0;
	}
	.actions {
		margin: 1rem 0;
	}
	.sev-urgent {
		background: #fde2e1;
		color: #7a1010;
	}
	.sev-warning {
		background: #fff1d6;
		color: #6b4300;
	}
</style>
