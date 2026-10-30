<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import {
		api,
		type ApiError,
		type MaintenanceEvent,
		type MaintenancePlan,
		type Page,
		type Tank
	} from '#lib/api/client.js';
	import Alert from '#lib/components/Alert.svelte';
	import MaintenanceDoneDialog from '#lib/components/MaintenanceDoneDialog.svelte';
	import { errorMessage } from '#lib/errors.js';
	import { formatDate, formatDateTime } from '#lib/format.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import { session } from '#lib/session.svelte.js';

	const id = $derived(page.params.id ?? '');
	const locale = getLocale();
	const tz = $derived(session.user?.timezone ?? 'Asia/Ho_Chi_Minh');

	let tank = $state<Tank | null>(null);
	let plans = $state<MaintenancePlan[]>([]);
	let events = $state<MaintenanceEvent[]>([]);
	let cursor = $state<string | null>(null);
	let loading = $state(true);
	let loadError = $state<ApiError | null>(null);
	let actionError = $state<ApiError | null>(null);
	let busy = $state(false);

	const canEdit = $derived(tank?.access === 'owner' && tank.status !== 'archived');
	const due = $derived(plans.filter((p) => p.active && (p.overdue || p.due_today)));
	const base = $derived(`/tanks/${encodeURIComponent(id)}`);

	async function load() {
		loading = true;
		loadError = null;
		const [t, p, e] = await Promise.all([
			api<Tank>('GET', base),
			api<Page<MaintenancePlan>>('GET', `${base}/maintenance-plans`),
			api<Page<MaintenanceEvent>>('GET', `${base}/maintenance-events?limit=20`)
		]);
		loading = false;
		for (const r of [t, p, e]) if (!r.ok) return void (loadError = r.error);
		if (!t.ok || !p.ok || !e.ok) return;
		tank = t.data;
		plans = p.data.items;
		events = e.data.items;
		cursor = e.data.next_cursor;
	}

	$effect(() => {
		void id;
		load();
	});

	async function more() {
		if (!cursor) return;
		const r = await api<Page<MaintenanceEvent>>(
			'GET',
			`${base}/maintenance-events?limit=20&cursor=${encodeURIComponent(cursor)}`
		);
		if (!r.ok) return void (actionError = r.error);
		events = [...events, ...r.data.items];
		cursor = r.data.next_cursor;
	}

	let target = $state<MaintenancePlan | null>(null);
	let doneOpen = $state(false);

	async function setActive(p: MaintenancePlan, active: boolean) {
		busy = true;
		actionError = null;
		const r = await api<MaintenancePlan>('PATCH', `/maintenance-plans/${p.id}`, {
			version: p.version,
			active
		});
		busy = false;
		if (!r.ok) return void (actionError = r.error);
		await load();
	}

	function dueText(p: MaintenancePlan): string {
		if (!p.active) return m.mt_inactive();
		if (p.overdue) return m.mt_overdue_since({ date: formatDate(p.next_due_on, locale) });
		if (p.due_today) return m.mt_due_today();
		return m.mt_next_due({ date: formatDate(p.next_due_on, locale) });
	}
</script>

<svelte:head
	><title>{m.tank_section_maintenance()} · {tank?.name ?? ''} · {m.app_name()}</title></svelte:head
>

<p><a href={resolve('/(app)/tanks/[id]', { id })}>← {tank?.name ?? m.nav_tanks()}</a></p>
<h1>{m.tank_section_maintenance()}</h1>

{#snippet planItem(p: MaintenancePlan, showDone: boolean)}
	<li class="item">
		<div class="row">
			<strong>{p.title}</strong>
			{#if p.overdue}<span class="badge">⚠ {m.wc_status_overdue()}</span>{/if}
			{#if p.due_today}<span class="badge">{m.mt_due_today()}</span>{/if}
			{#if p.equipment_name}<span class="badge">{p.equipment_name}</span>{/if}
		</div>
		<p>{m.mt_every_days({ n: String(p.interval_days) })} · {dueText(p)}</p>
		{#if p.last_performed_at}
			<p class="muted">
				{m.mt_last_done({ when: formatDateTime(p.last_performed_at, locale, tz) })}
			</p>
		{/if}
		{#if p.notes}<p class="notes">{p.notes}</p>{/if}
		{#if canEdit}
			<div class="actions">
				{#if showDone}
					<button
						type="button"
						onclick={() => {
							target = p;
							doneOpen = true;
						}}>{m.mt_done()}</button
					>
				{/if}
				<a
					class="button secondary"
					href={resolve('/(app)/tanks/[id]/maintenance/plans/[planId]', { id, planId: p.id })}
					>{m.wc_edit()}</a
				>
				{#if p.active}
					<button
						type="button"
						class="secondary"
						disabled={busy}
						onclick={() => setActive(p, false)}>{m.mt_deactivate()}</button
					>
				{:else}
					<button type="button" class="secondary" disabled={busy} onclick={() => setActive(p, true)}
						>{m.mt_activate()}</button
					>
				{/if}
			</div>
		{/if}
	</li>
{/snippet}

{#if loading && !tank}
	<p role="status">{m.loading()}</p>
{:else if loadError}
	{#if loadError.code === 'not_found'}
		<p>{m.not_found_body()}</p>
	{:else}
		<Alert kind="error">
			{errorMessage(loadError)}
			<button type="button" class="secondary" onclick={load}>{m.retry()}</button>
		</Alert>
	{/if}
{:else if tank}
	{#if tank.access === 'viewer'}<Alert kind="info">{m.viewer_notice()}</Alert>{/if}
	{#if tank.status === 'archived'}<Alert kind="info">{m.archived_notice()}</Alert>{/if}
	{#if actionError}<Alert kind="error">{errorMessage(actionError)}</Alert>{/if}

	<div class="actions">
		{#if canEdit}
			<a class="button" href={resolve('/(app)/tanks/[id]/maintenance/plans/new', { id })}
				>{m.mt_plan_new()}</a
			>
			<a class="button secondary" href={resolve('/(app)/tanks/[id]/maintenance/record', { id })}
				>{m.mt_record()}</a
			>
		{/if}
		<a class="button secondary" href={resolve('/(app)/tanks/[id]/equipment', { id })}
			>{m.eq_title()}</a
		>
	</div>

	{#if due.length > 0}
		<section class="card" aria-labelledby="due">
			<h2 id="due">{m.mt_due_title()} ({due.length})</h2>
			<p class="muted">{m.mt_due_hint()}</p>
			<ul class="list">
				{#each due as p (p.id)}{@render planItem(p, true)}{/each}
			</ul>
		</section>
	{/if}

	<section class="card" aria-labelledby="plans">
		<h2 id="plans">{m.mt_plans_title()}</h2>
		{#if plans.length === 0}
			<p class="muted">{m.mt_plans_empty()}</p>
		{:else}
			<ul class="list">
				{#each plans as p (p.id)}{@render planItem(p, !(p.overdue || p.due_today))}{/each}
			</ul>
		{/if}
	</section>

	<section class="card" aria-labelledby="history">
		<h2 id="history">{m.wc_history_title()}</h2>
		{#if events.length === 0}
			<p class="muted">{m.mt_history_empty()}</p>
		{:else}
			<ul class="list">
				{#each events as ev (ev.id)}
					<li class="item">
						<div class="row">
							<strong>{ev.title}</strong>
							{#if !ev.plan_id}<span class="badge">{m.dose_ad_hoc()}</span>{/if}
						</div>
						<p class="muted">
							<time datetime={ev.performed_at}>{formatDateTime(ev.performed_at, locale, tz)}</time>
						</p>
						{#if ev.notes}<p class="notes">{ev.notes}</p>{/if}
					</li>
				{/each}
			</ul>
			{#if cursor}
				<button type="button" class="secondary" onclick={more}>{m.load_more()}</button>
			{/if}
		{/if}
	</section>

	<MaintenanceDoneDialog bind:open={doneOpen} plan={target} timeZone={tz} ondone={load} />
{/if}

<style>
	.list {
		list-style: none;
		margin: 0;
		padding: 0;
	}
	.item {
		border-top: 1px solid var(--border, #dde);
		padding: 0.75rem 0;
	}
	.item:first-child {
		border-top: 0;
	}
	.row {
		display: flex;
		flex-wrap: wrap;
		gap: 0.4rem 0.75rem;
		align-items: center;
	}
	.item p {
		margin: 0.25rem 0;
	}
	.notes {
		white-space: pre-line;
	}
</style>
