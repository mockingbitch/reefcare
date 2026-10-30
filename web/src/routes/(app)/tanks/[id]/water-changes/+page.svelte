<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import {
		api,
		type ApiError,
		type Page,
		type Tank,
		type WaterChange,
		type WaterChangeSchedule
	} from '#lib/api/client.js';
	import Alert from '#lib/components/Alert.svelte';
	import ConfirmDialog from '#lib/components/ConfirmDialog.svelte';
	import EstimateNote from '#lib/components/EstimateNote.svelte';
	import Field from '#lib/components/Field.svelte';
	import { errorMessage, fieldErrors } from '#lib/errors.js';
	import { formatDate, formatDateTime, formatDecimal } from '#lib/format.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import { describeRRule } from '#lib/recurrence.js';
	import { session } from '#lib/session.svelte.js';
	import { decimalInput } from '#lib/tank-form.js';
	import { fromLocalInput, toLocalInput } from '#lib/water-changes.js';

	const id = $derived(page.params.id ?? '');
	const locale = getLocale();
	const tz = $derived(session.user?.timezone ?? 'Asia/Ho_Chi_Minh');
	const dec = (v: string) => formatDecimal(v, locale);

	let tank = $state<Tank | null>(null);
	let schedules = $state<WaterChangeSchedule[]>([]);
	let overdue = $state<WaterChange[]>([]);
	let upcoming = $state<WaterChange[]>([]);
	let history = $state<WaterChange[]>([]);
	let historyCursor = $state<string | null>(null);
	let loading = $state(true);
	let loadError = $state<ApiError | null>(null);
	let actionError = $state<ApiError | null>(null);

	const canEdit = $derived(tank?.access === 'owner' && tank.status !== 'archived');
	const base = $derived(`/tanks/${encodeURIComponent(id)}`);

	async function load() {
		loading = true;
		loadError = null;
		const [t, s, o, u, h] = await Promise.all([
			api<Tank>('GET', base),
			api<Page<WaterChangeSchedule>>('GET', `${base}/water-change-schedules`),
			api<Page<WaterChange>>('GET', `${base}/water-changes?status=overdue&limit=50`),
			api<Page<WaterChange>>('GET', `${base}/water-changes?status=scheduled&limit=50`),
			api<Page<WaterChange>>('GET', `${base}/water-changes?status=completed,skipped&limit=20`)
		]);
		loading = false;
		for (const r of [t, s, o, u, h]) {
			if (!r.ok) {
				loadError = r.error;
				return;
			}
		}
		if (!t.ok || !s.ok || !o.ok || !u.ok || !h.ok) return;
		tank = t.data;
		schedules = s.data.items;
		overdue = o.data.items;
		// The list is newest first; show the soonest upcoming first.
		upcoming = [...u.data.items].reverse().slice(0, 5);
		history = h.data.items;
		historyCursor = h.data.next_cursor;
	}

	$effect(() => {
		void id;
		load();
	});

	async function moreHistory() {
		if (!historyCursor) return;
		const r = await api<Page<WaterChange>>(
			'GET',
			`${base}/water-changes?status=completed,skipped&limit=20&cursor=${encodeURIComponent(historyCursor)}`
		);
		if (!r.ok) {
			actionError = r.error;
			return;
		}
		history = [...history, ...r.data.items];
		historyCursor = r.data.next_cursor;
	}

	// --- Complete / skip -------------------------------------------------------------------
	let target = $state<WaterChange | null>(null);
	let completeOpen = $state(false);
	let skipOpen = $state(false);
	let busy = $state(false);
	let dialogError = $state<ApiError | null>(null);
	const dfe = $derived(fieldErrors(dialogError));
	let completedAt = $state('');
	let actualVolume = $state('');
	let saltMix = $state('');
	let notes = $state('');
	let reason = $state('');

	function openComplete(wc: WaterChange) {
		target = wc;
		dialogError = null;
		completedAt = toLocalInput(new Date().toISOString(), tz);
		actualVolume = wc.planned_volume_l ?? '';
		saltMix = wc.salt_mix ?? '';
		notes = '';
		completeOpen = true;
	}

	function openSkip(wc: WaterChange) {
		target = wc;
		dialogError = null;
		reason = '';
		skipOpen = true;
	}

	async function complete() {
		if (!target) return;
		busy = true;
		dialogError = null;
		const r = await api<WaterChange>('POST', `/water-changes/${target.id}/complete`, {
			completed_at: fromLocalInput(completedAt, tz) || undefined,
			actual_volume_l: decimalInput(actualVolume),
			salt_mix: saltMix.trim() || null,
			notes: notes.trim() || null
		});
		busy = false;
		if (!r.ok) {
			dialogError = r.error;
			return;
		}
		completeOpen = false;
		await load();
	}

	async function skip() {
		if (!target) return;
		busy = true;
		dialogError = null;
		const r = await api<WaterChange>('POST', `/water-changes/${target.id}/skip`, {
			reason: reason.trim()
		});
		busy = false;
		if (!r.ok) {
			dialogError = r.error;
			return;
		}
		skipOpen = false;
		await load();
	}

	// --- Schedule status -------------------------------------------------------------------
	let ending = $state<WaterChangeSchedule | null>(null);
	let endOpen = $state(false);

	async function setStatus(s: WaterChangeSchedule, status: 'active' | 'paused' | 'ended') {
		busy = true;
		actionError = null;
		const r = await api<WaterChangeSchedule>('PATCH', `/water-change-schedules/${s.id}`, {
			version: s.version,
			status
		});
		busy = false;
		endOpen = false;
		if (!r.ok) {
			actionError = r.error;
			return;
		}
		await load();
	}

	const scheduleStatus: Record<string, () => string> = {
		active: m.wc_schedule_status_active,
		paused: m.wc_schedule_status_paused,
		ended: m.wc_schedule_status_ended
	};

	function when(wc: WaterChange): string {
		const at = wc.completed_at ?? wc.scheduled_for;
		return at ? formatDateTime(at, locale, tz) : '';
	}
</script>

<svelte:head><title>{m.wc_title()} · {tank?.name ?? ''} · {m.app_name()}</title></svelte:head>

<p>
	<a href={resolve('/(app)/tanks/[id]', { id })}>← {tank?.name ?? m.nav_tanks()}</a>
</p>
<h1>{m.wc_title()}</h1>

{#snippet item(wc: WaterChange, actions: boolean)}
	<li class="item">
		<div class="row">
			<time datetime={wc.completed_at ?? wc.scheduled_for ?? undefined}>{when(wc)}</time>
			{#if wc.overdue}
				<span class="badge warn">⚠ {m.wc_status_overdue()}</span>
			{:else if wc.status === 'completed'}
				<span class="badge">✓ {m.wc_status_completed()}</span>
			{:else if wc.status === 'skipped'}
				<span class="badge">↷ {m.wc_status_skipped()}</span>
			{:else}
				<span class="badge">{m.wc_status_scheduled()}</span>
			{/if}
			{#if !wc.schedule_id}<span class="badge">{m.wc_one_time()}</span>{/if}
		</div>
		<p>
			{#if wc.status === 'completed' && wc.actual_volume_l}
				{m.wc_actual_volume({ volume: dec(wc.actual_volume_l) })}
			{:else if wc.planned_volume_l}
				{m.wc_planned_volume({ volume: dec(wc.planned_volume_l) })}
			{/if}
			{#if wc.salt_mix}· {wc.salt_mix}{/if}
		</p>
		{#if wc.skip_reason}<p>{m.wc_skipped_reason({ reason: wc.skip_reason })}</p>{/if}
		{#if wc.notes}<p class="notes">{wc.notes}</p>{/if}
		{#if wc.estimate}<EstimateNote estimate={wc.estimate} />{/if}
		{#if actions && canEdit}
			<div class="actions">
				<button type="button" onclick={() => openComplete(wc)}>{m.wc_complete()}</button>
				<button type="button" class="secondary" onclick={() => openSkip(wc)}>{m.wc_skip()}</button>
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

	{#if canEdit}
		<div class="actions">
			<a class="button" href={resolve('/(app)/tanks/[id]/water-changes/record', { id })}
				>{m.wc_record()}</a
			>
			<a
				class="button secondary"
				href={resolve('/(app)/tanks/[id]/water-changes/schedules/new', { id })}
				>{m.wc_schedule_new()}</a
			>
		</div>
	{/if}

	{#if overdue.length > 0}
		<section class="card" aria-labelledby="overdue">
			<h2 id="overdue">{m.wc_overdue_title()} ({overdue.length})</h2>
			<p class="muted">{m.wc_overdue_hint()}</p>
			<ul class="list">
				{#each overdue as wc (wc.id)}{@render item(wc, true)}{/each}
			</ul>
		</section>
	{/if}

	<section class="card" aria-labelledby="upcoming">
		<h2 id="upcoming">{m.wc_upcoming_title()}</h2>
		{#if upcoming.length === 0}
			<p class="muted">{m.wc_upcoming_empty()}</p>
		{:else}
			<ul class="list">
				{#each upcoming as wc (wc.id)}{@render item(wc, true)}{/each}
			</ul>
		{/if}
	</section>

	<section class="card" aria-labelledby="schedules">
		<h2 id="schedules">{m.wc_schedules_title()}</h2>
		{#if schedules.length === 0}
			<p class="muted">{m.wc_schedules_empty()}</p>
		{:else}
			<ul class="list">
				{#each schedules as s (s.id)}
					<li class="item">
						<div class="row">
							<strong>{describeRRule(s.rrule)}</strong>
							<span class="badge">{scheduleStatus[s.status]?.() ?? s.status}</span>
						</div>
						<p>
							{m.wc_planned_volume({ volume: dec(s.planned_volume_l) })}
							· {m.wc_starts_on_label()}
							{formatDate(s.starts_on, locale)}{#if s.ends_on}
								· {m.wc_ends_on_label()} {formatDate(s.ends_on, locale)}{/if}
						</p>
						{#if s.status === 'active'}
							<p class="muted">{m.wc_next_dates()}:</p>
							{#if s.next_occurrences.length === 0}
								<p>{m.wc_no_next()}</p>
							{:else}
								<ol>
									{#each s.next_occurrences as at (at)}
										<li><time datetime={at}>{formatDateTime(at, locale, s.timezone)}</time></li>
									{/each}
								</ol>
							{/if}
						{/if}
						{#if s.estimate}<EstimateNote estimate={s.estimate} />{/if}
						{#if canEdit && s.status !== 'ended'}
							<div class="actions">
								<a
									class="button secondary"
									href={resolve('/(app)/tanks/[id]/water-changes/schedules/[scheduleId]', {
										id,
										scheduleId: s.id
									})}>{m.wc_edit()}</a
								>
								{#if s.status === 'active'}
									<button
										type="button"
										class="secondary"
										disabled={busy}
										onclick={() => setStatus(s, 'paused')}>{m.wc_pause()}</button
									>
								{:else}
									<button
										type="button"
										class="secondary"
										disabled={busy}
										onclick={() => setStatus(s, 'active')}>{m.wc_resume()}</button
									>
								{/if}
								<button
									type="button"
									class="secondary"
									onclick={() => {
										ending = s;
										endOpen = true;
									}}>{m.wc_end()}</button
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
			<p class="muted">{m.wc_history_empty()}</p>
		{:else}
			<ul class="list">
				{#each history as wc (wc.id)}{@render item(wc, false)}{/each}
			</ul>
			{#if historyCursor}
				<button type="button" class="secondary" onclick={moreHistory}>{m.load_more()}</button>
			{/if}
		{/if}
	</section>

	<ConfirmDialog
		bind:open={completeOpen}
		title={m.wc_complete_title()}
		confirmLabel={m.wc_complete()}
		{busy}
		onconfirm={complete}
	>
		{#if dialogError && dialogError.code !== 'validation_failed'}
			<Alert kind="error">{errorMessage(dialogError)}</Alert>
		{/if}
		<Field
			label={m.wc_completed_at_label()}
			name="completed_at"
			type="datetime-local"
			bind:value={completedAt}
			hint={m.rec_timezone_note({ tz })}
			error={dfe.completed_at}
		/>
		<Field
			label={m.wc_actual_volume_label()}
			name="actual_volume_l"
			inputmode="decimal"
			required
			bind:value={actualVolume}
			error={dfe.actual_volume_l}
		/>
		<Field
			label={m.field_salt_mix()}
			name="salt_mix"
			optional
			bind:value={saltMix}
			error={dfe.salt_mix}
		/>
		<Field
			label={m.wc_notes_label()}
			name="notes"
			optional
			multiline
			bind:value={notes}
			error={dfe.notes}
		/>
	</ConfirmDialog>

	<ConfirmDialog
		bind:open={skipOpen}
		title={m.wc_skip_title()}
		confirmLabel={m.wc_skip()}
		{busy}
		onconfirm={skip}
	>
		{#if dialogError && dialogError.code !== 'validation_failed'}
			<Alert kind="error">{errorMessage(dialogError)}</Alert>
		{/if}
		<Field
			label={m.wc_skip_reason_label()}
			name="reason"
			required
			bind:value={reason}
			error={dfe.reason}
		/>
	</ConfirmDialog>

	<ConfirmDialog
		bind:open={endOpen}
		title={m.wc_end_confirm_title()}
		confirmLabel={m.wc_end()}
		danger
		{busy}
		onconfirm={() => ending && setStatus(ending, 'ended')}
	>
		<p>{m.wc_end_confirm_body()}</p>
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
		gap: 0.4rem;
		align-items: center;
	}
	.item p {
		margin: 0.25rem 0;
	}
	.notes {
		white-space: pre-line;
	}
	.warn {
		font-weight: 600;
	}
	ol {
		margin: 0.25rem 0;
		padding-left: 1.25rem;
	}
</style>
