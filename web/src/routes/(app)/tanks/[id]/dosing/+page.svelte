<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import {
		api,
		type ApiError,
		type DoseEvent,
		type DosingPlan,
		type Page,
		type Tank
	} from '#lib/api/client.js';
	import Alert from '#lib/components/Alert.svelte';
	import ConfirmDialog from '#lib/components/ConfirmDialog.svelte';
	import DoseResolveDialog from '#lib/components/DoseResolveDialog.svelte';
	import { doseText, dueSoon } from '#lib/dosing.js';
	import { errorMessage } from '#lib/errors.js';
	import { formatDate, formatDateTime } from '#lib/format.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import { describeRRule } from '#lib/recurrence.js';
	import { session } from '#lib/session.svelte.js';

	const id = $derived(page.params.id ?? '');
	const locale = getLocale();
	const tz = $derived(session.user?.timezone ?? 'Asia/Ho_Chi_Minh');

	let tank = $state<Tank | null>(null);
	let plans = $state<DosingPlan[]>([]);
	let attention = $state<DoseEvent[]>([]);
	let upcoming = $state<DoseEvent[]>([]);
	let history = $state<DoseEvent[]>([]);
	let historyCursor = $state<string | null>(null);
	let loading = $state(true);
	let loadError = $state<ApiError | null>(null);
	let actionError = $state<ApiError | null>(null);
	let busy = $state(false);

	const canEdit = $derived(tank?.access === 'owner' && tank.status !== 'archived');
	const base = $derived(`/tanks/${encodeURIComponent(id)}`);

	function upcomingWindow(): string {
		const now = Date.now();
		return new URLSearchParams({
			status: 'scheduled',
			from: new Date(now - 3_600_000).toISOString(),
			to: new Date(now + 48 * 3_600_000).toISOString(),
			limit: '200'
		}).toString();
	}

	async function load() {
		loading = true;
		loadError = null;
		const [t, p, a, u, h] = await Promise.all([
			api<Tank>('GET', base),
			api<Page<DosingPlan>>('GET', `${base}/dosing-plans`),
			api<Page<DoseEvent>>('GET', `${base}/dosing-history?status=missed,unconfirmed&limit=50`),
			// Only the next 48 h: the list is newest first, and busy pump plans could otherwise
			// push the soonest doses out of a page.
			api<Page<DoseEvent>>('GET', `${base}/dosing-history?${upcomingWindow()}`),
			api<Page<DoseEvent>>(
				'GET',
				`${base}/dosing-history?status=completed,partial,skipped,failed&limit=20`
			)
		]);
		loading = false;
		for (const r of [t, p, a, u, h]) {
			if (!r.ok) {
				loadError = r.error;
				return;
			}
		}
		if (!t.ok || !p.ok || !a.ok || !u.ok || !h.ok) return;
		tank = t.data;
		plans = p.data.items;
		attention = a.data.items;
		// Newest first from the API; show the soonest doses first.
		upcoming = [...u.data.items].reverse().slice(0, 10);
		history = h.data.items;
		historyCursor = h.data.next_cursor;
	}

	$effect(() => {
		void id;
		load();
	});

	async function moreHistory() {
		if (!historyCursor) return;
		const r = await api<Page<DoseEvent>>(
			'GET',
			`${base}/dosing-history?status=completed,partial,skipped,failed&limit=20&cursor=${encodeURIComponent(historyCursor)}`
		);
		if (!r.ok) {
			actionError = r.error;
			return;
		}
		history = [...history, ...r.data.items];
		historyCursor = r.data.next_cursor;
	}

	// --- Resolve one dose --------------------------------------------------------------------
	let resolving = $state<DoseEvent | null>(null);
	let resolveOpen = $state(false);
	function openResolve(d: DoseEvent) {
		resolving = d;
		resolveOpen = true;
	}

	// --- Plan status -------------------------------------------------------------------------
	let completing = $state<DosingPlan | null>(null);
	let completeOpen = $state(false);

	async function planAction(p: DosingPlan, action: 'pause' | 'resume' | 'complete') {
		busy = true;
		actionError = null;
		const r = await api<DosingPlan>('POST', `/dosing-plans/${p.id}/${action}`);
		busy = false;
		completeOpen = false;
		if (!r.ok) {
			actionError = r.error;
			return;
		}
		await load();
	}

	const planStatus: Record<string, () => string> = {
		active: m.wc_schedule_status_active,
		paused: m.wc_schedule_status_paused,
		completed: m.dose_plan_status_completed
	};

	const statusLabels: Record<string, () => string> = {
		scheduled: m.dose_status_scheduled,
		completed: m.dose_status_completed,
		partial: m.dose_status_partial,
		skipped: m.dose_status_skipped,
		failed: m.dose_status_failed,
		missed: m.dose_status_missed,
		unconfirmed: m.dose_status_unconfirmed
	};
	const statusIcon: Record<string, string> = {
		completed: '✓',
		partial: '◐',
		skipped: '↷',
		failed: '✕',
		missed: '⚠',
		unconfirmed: '?'
	};

	function amountOf(d: DoseEvent): string {
		const a = d.actual_amount ?? d.planned_amount;
		return a ? doseText(a, d.amount_unit, locale) : '';
	}
</script>

<svelte:head><title>{m.dose_title()} · {tank?.name ?? ''} · {m.app_name()}</title></svelte:head>

<p><a href={resolve('/(app)/tanks/[id]', { id })}>← {tank?.name ?? m.nav_tanks()}</a></p>
<h1>{m.dose_title()}</h1>

{#snippet dose(d: DoseEvent, actionable: boolean)}
	<li class="item">
		<div class="row">
			<strong>{d.product.name}</strong>
			<span>{amountOf(d)}</span>
			<span class="badge">
				{#if statusIcon[d.status]}<span aria-hidden="true">{statusIcon[d.status]}</span>{/if}
				{statusLabels[d.status]?.() ?? d.status}
			</span>
			{#if !d.plan_id}<span class="badge">{m.dose_ad_hoc()}</span>{/if}
		</div>
		<p class="muted">
			{#if d.executed_at}
				<time datetime={d.executed_at}>{formatDateTime(d.executed_at, locale, tz)}</time>
			{:else if d.scheduled_for}
				<time datetime={d.scheduled_for}>{formatDateTime(d.scheduled_for, locale, tz)}</time>
			{/if}
			{#if d.deviation === 'over'}· {m.dose_deviation_over()}{/if}
			{#if d.deviation === 'under'}· {m.dose_deviation_under()}{/if}
		</p>
		{#if d.reason}<p>{m.dose_reason({ reason: d.reason })}</p>{/if}
		{#if d.notes}<p class="notes">{d.notes}</p>{/if}
		{#if actionable && canEdit}
			{#if d.status !== 'scheduled'}
				<button type="button" onclick={() => openResolve(d)}>{m.dose_record_what_happened()}</button
				>
			{:else if dueSoon(d)}
				<button type="button" onclick={() => openResolve(d)}>
					{m.dose_confirm_button({
						dose: doseText(d.planned_amount ?? '', d.amount_unit, locale)
					})}
				</button>
			{:else}
				<button type="button" class="secondary" onclick={() => openResolve(d)}
					>{m.dose_record_early()}</button
				>
			{/if}
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

	{#if canEdit}
		<div class="actions">
			<a class="button" href={resolve('/(app)/tanks/[id]/dosing/record', { id })}
				>{m.dose_record_ad_hoc()}</a
			>
			<a class="button secondary" href={resolve('/(app)/tanks/[id]/dosing/plans/new', { id })}
				>{m.dose_plan_new()}</a
			>
			<a class="button secondary" href={resolve('/(app)/dosing/products')}
				>{m.dose_products_manage()}</a
			>
		</div>
	{/if}

	{#if attention.length > 0}
		<section class="card" aria-labelledby="attention">
			<h2 id="attention">{m.dose_attention_title()} ({attention.length})</h2>
			<p class="muted">{m.dose_attention_hint()}</p>
			<ul class="list">
				{#each attention as d (d.id)}{@render dose(d, true)}{/each}
			</ul>
		</section>
	{/if}

	<section class="card" aria-labelledby="upcoming">
		<h2 id="upcoming">{m.wc_upcoming_title()}</h2>
		{#if upcoming.length === 0}
			<p class="muted">{m.dose_upcoming_empty()}</p>
		{:else}
			<ul class="list">
				{#each upcoming as d (d.id)}{@render dose(d, true)}{/each}
			</ul>
		{/if}
	</section>

	<section class="card" aria-labelledby="plans">
		<h2 id="plans">{m.dose_plans_title()}</h2>
		{#if plans.length === 0}
			<p class="muted">{m.dose_plans_empty()}</p>
		{:else}
			<ul class="list">
				{#each plans as p (p.id)}
					<li class="item">
						<div class="row">
							<strong>{p.product.name}</strong>
							<span>{doseText(p.amount, p.amount_unit, locale)}</span>
							<span class="badge">{planStatus[p.status]?.() ?? p.status}</span>
							{#if p.delivery_method === 'dosing_pump'}
								<span class="badge">{m.dose_delivery_pump()}</span>
							{/if}
						</div>
						<p>
							{describeRRule(p.rrule)} · {m.wc_starts_on_label()}
							{formatDate(p.starts_on, locale)}{#if p.ends_on}
								· {m.wc_ends_on_label()} {formatDate(p.ends_on, locale)}{/if}
						</p>
						{#if p.status === 'active' && p.next_occurrences.length > 0}
							<p class="muted">
								{m.wc_next_dates()}:
								{#each p.next_occurrences as at, i (at)}
									{#if i > 0},
									{/if}<time datetime={at}>{formatDateTime(at, locale, p.timezone)}</time>
								{/each}
							</p>
						{/if}
						{#if canEdit && p.status !== 'completed'}
							<div class="actions">
								<a
									class="button secondary"
									href={resolve('/(app)/tanks/[id]/dosing/plans/[planId]', {
										id,
										planId: p.id
									})}>{m.wc_edit()}</a
								>
								{#if p.status === 'active'}
									<button
										type="button"
										class="secondary"
										disabled={busy}
										onclick={() => planAction(p, 'pause')}>{m.wc_pause()}</button
									>
								{:else}
									<button
										type="button"
										class="secondary"
										disabled={busy}
										onclick={() => planAction(p, 'resume')}>{m.wc_resume()}</button
									>
								{/if}
								<button
									type="button"
									class="secondary"
									onclick={() => {
										completing = p;
										completeOpen = true;
									}}>{m.dose_plan_complete()}</button
								>
							</div>
						{/if}
					</li>
				{/each}
			</ul>
		{/if}
	</section>

	<section class="card" aria-labelledby="history">
		<h2 id="history">{m.wc_history_title()}</h2>
		{#if history.length === 0}
			<p class="muted">{m.dose_history_empty()}</p>
		{:else}
			<ul class="list">
				{#each history as d (d.id)}{@render dose(d, false)}{/each}
			</ul>
			{#if historyCursor}
				<button type="button" class="secondary" onclick={moreHistory}>{m.load_more()}</button>
			{/if}
		{/if}
	</section>

	<DoseResolveDialog
		bind:open={resolveOpen}
		dose={resolving}
		tankId={id}
		{plans}
		timeZone={tz}
		onresolved={load}
	/>

	<ConfirmDialog
		bind:open={completeOpen}
		title={m.dose_plan_complete_title()}
		confirmLabel={m.dose_plan_complete()}
		danger
		{busy}
		onconfirm={() => completing && planAction(completing, 'complete')}
	>
		<p>{m.dose_plan_complete_body()}</p>
	</ConfirmDialog>
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
