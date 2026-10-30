<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import {
		api,
		type ApiError,
		type LivestockEntry,
		type LivestockEvent,
		type Page,
		type Tank
	} from '#lib/api/client.js';
	import Alert from '#lib/components/Alert.svelte';
	import ConfirmDialog from '#lib/components/ConfirmDialog.svelte';
	import Field from '#lib/components/Field.svelte';
	import { errorMessage, fieldErrors } from '#lib/errors.js';
	import { formatDateTime } from '#lib/format.js';
	import {
		arrivalTypes,
		coralTypeLabel,
		deltaText,
		eventLabel,
		isObservation,
		livestockCategoryLabel,
		livestockFormLabel,
		lossTypes,
		observationTypes,
		quantityIsApproximate,
		speciesName,
		type RecordableEvent
	} from '#lib/livestock.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import { session } from '#lib/session.svelte.js';
	import { fromLocalInput, toLocalInput } from '#lib/water-changes.js';

	const id = $derived(page.params.id ?? '');
	const entryId = $derived(page.params.entryId ?? '');
	const locale = getLocale();
	const tz = $derived(session.user?.timezone ?? 'Asia/Ho_Chi_Minh');

	let tank = $state<Tank | null>(null);
	let entry = $state<LivestockEntry | null>(null);
	let events = $state<LivestockEvent[]>([]);
	let cursor = $state<string | null>(null);
	let loading = $state(true);
	let loadError = $state<ApiError | null>(null);

	const canEdit = $derived(tank?.access === 'owner' && tank.status !== 'archived');

	async function load() {
		loading = true;
		loadError = null;
		const eid = encodeURIComponent(entryId);
		const [t, e, ev] = await Promise.all([
			api<Tank>('GET', `/tanks/${encodeURIComponent(id)}`),
			api<LivestockEntry>('GET', `/livestock/${eid}`),
			api<Page<LivestockEvent>>('GET', `/livestock/${eid}/events?limit=20`)
		]);
		loading = false;
		for (const r of [t, e, ev]) if (!r.ok) return void (loadError = r.error);
		if (!t.ok || !e.ok || !ev.ok) return;
		tank = t.data;
		entry = e.data;
		events = ev.data.items;
		cursor = ev.data.next_cursor;
	}

	$effect(() => {
		void entryId;
		load();
	});

	async function more() {
		if (!cursor) return;
		const r = await api<Page<LivestockEvent>>(
			'GET',
			`/livestock/${encodeURIComponent(entryId)}/events?limit=20&cursor=${encodeURIComponent(cursor)}`
		);
		if (r.ok) {
			events = [...events, ...r.data.items];
			cursor = r.data.next_cursor;
		}
	}

	// --- Record an event --------------------------------------------------------------------------
	let type = $state<RecordableEvent>('died');
	let quantity = $state<string | number>('1');
	let occurredAt = $state('');
	let note = $state('');
	let busy = $state(false);
	let eventError = $state<ApiError | null>(null);
	let saved = $state('');
	const efe = $derived(fieldErrors(eventError));
	let eventKey = crypto.randomUUID();

	$effect(() => {
		if (!occurredAt) occurredAt = toLocalInput(new Date().toISOString(), tz);
	});

	const typeHints: Partial<Record<RecordableEvent, () => string>> = {
		missing: m.ls_hint_missing,
		transferred_out: m.ls_hint_transferred_out,
		propagated: m.ls_hint_propagated,
		health_observation: m.ls_hint_observation,
		treatment: m.ls_hint_observation
	};

	async function recordEvent() {
		busy = true;
		eventError = null;
		saved = '';
		const body: Record<string, unknown> = {
			type,
			occurred_at: fromLocalInput(occurredAt, tz) || undefined,
			note: note.trim() || null
		};
		if (!isObservation(type)) body.quantity = Number(String(quantity).trim()) || 0;
		const r = await api<LivestockEvent>(
			'POST',
			`/livestock/${encodeURIComponent(entryId)}/events`,
			body,
			fetch,
			{ 'Idempotency-Key': eventKey }
		);
		busy = false;
		if (!r.ok) return void (eventError = r.error);
		saved = eventLabel(r.data.type);
		eventKey = crypto.randomUUID(); // the next event is a new one
		note = '';
		quantity = '1';
		occurredAt = toLocalInput(new Date().toISOString(), tz);
		await load();
	}

	// --- Reconcile ---------------------------------------------------------------------------------
	let reconcileOpen = $state(false);
	let count = $state<string | number>('');
	let reconcileAt = $state('');
	let reconcileNote = $state('');
	let reconcileError = $state<ApiError | null>(null);
	const rfe = $derived(fieldErrors(reconcileError));
	let reconcileKey = '';

	function openReconcile() {
		count = String(entry?.current_quantity ?? 0);
		reconcileAt = toLocalInput(new Date().toISOString(), tz);
		reconcileNote = '';
		reconcileError = null;
		reconcileKey = crypto.randomUUID();
		reconcileOpen = true;
	}

	async function reconcile() {
		busy = true;
		reconcileError = null;
		const r = await api<LivestockEntry>(
			'POST',
			`/livestock/${encodeURIComponent(entryId)}/reconcile`,
			{
				count: Number(String(count).trim()),
				occurred_at: fromLocalInput(reconcileAt, tz) || undefined,
				note: reconcileNote.trim() || null
			},
			fetch,
			{ 'Idempotency-Key': reconcileKey }
		);
		busy = false;
		if (!r.ok) return void (reconcileError = r.error);
		reconcileOpen = false;
		await load();
	}
</script>

<svelte:head><title>{entry?.display_name ?? m.ls_title()} · {m.app_name()}</title></svelte:head>

<p><a href={resolve('/(app)/tanks/[id]/livestock', { id })}>← {m.ls_title()}</a></p>

{#if loading && !entry}
	<p role="status">{m.loading()}</p>
{:else if loadError}
	{#if loadError.code === 'not_found'}
		<h1>{m.not_found_title()}</h1>
		<p>{m.not_found_body()}</p>
	{:else}
		<Alert kind="error">
			{errorMessage(loadError)}
			<button type="button" class="secondary" onclick={load}>{m.retry()}</button>
		</Alert>
	{/if}
{:else if entry && tank}
	<h1>{entry.display_name}</h1>
	<p class="meta">
		<span class="badge">{livestockCategoryLabel(entry.category)}</span>
		{#if entry.coral_type}<span class="badge">{coralTypeLabel(entry.coral_type)}</span>{/if}
		{#if entry.form}<span class="badge">{livestockFormLabel(entry.form)}</span>{/if}
		{#if !entry.present}<span class="badge">{m.ls_gone_badge()}</span>{/if}
		{#if entry.quarantine_status === 'in_quarantine'}
			<span class="badge">⚠ {m.ls_quarantine_in()}</span>
		{:else if entry.quarantine_status === 'completed'}
			<span class="badge">{m.ls_quarantine_completed()}</span>
		{/if}
	</p>

	<section class="card" aria-labelledby="count">
		<h2 id="count">{m.ls_quantity_label()}</h2>
		<p class="big">{entry.current_quantity}</p>
		<p class="muted">
			{quantityIsApproximate(entry.category) ? m.ls_quantity_hint_approx() : m.ls_quantity_hint()}
		</p>
		{#if canEdit}
			<button type="button" class="secondary" onclick={openReconcile}>{m.ls_reconcile()}</button>
		{/if}
	</section>

	<section class="card" aria-labelledby="info">
		<h2 id="info">{m.tank_info()}</h2>
		<dl class="facts">
			<dt>{m.ls_species_label()}</dt>
			<dd>
				{#if entry.species}
					{speciesName(entry.species, locale)}{#if entry.species.scientific_name}
						· <em>{entry.species.scientific_name}</em>{/if}
				{:else}
					{m.not_set()}
				{/if}
			</dd>
			<dt>{m.ls_added_at_label()}</dt>
			<dd>{formatDateTime(entry.added_at, locale, tz)}</dd>
			<dt>{m.ls_location_label()}</dt>
			<dd>{entry.location_note ?? m.not_set()}</dd>
			<dt>{m.ls_source_label()}</dt>
			<dd>{entry.source_note ?? m.not_set()}</dd>
		</dl>
		{#if entry.notes}<p class="notes">{entry.notes}</p>{/if}
		{#if canEdit}
			<a
				class="button secondary"
				href={resolve('/(app)/tanks/[id]/livestock/[entryId]/edit', { id, entryId })}
				>{m.wc_edit()}</a
			>
		{/if}
	</section>

	{#if canEdit}
		<section class="card" aria-labelledby="record">
			<h2 id="record">{m.ls_record_title()}</h2>
			{#if saved}<Alert kind="success">{m.ls_recorded({ type: saved })}</Alert>{/if}
			{#if eventError && eventError.code !== 'validation_failed'}
				<Alert kind="error">{errorMessage(eventError)}</Alert>
			{/if}
			<form
				novalidate
				onsubmit={(e) => {
					e.preventDefault();
					recordEvent();
				}}
			>
				<div class="field">
					<label for="event-type">{m.ls_event_type_label()}</label>
					<select id="event-type" name="type" bind:value={type}>
						<optgroup label={m.ls_group_losses()}>
							{#each lossTypes as t (t)}<option value={t}>{eventLabel(t)}</option>{/each}
						</optgroup>
						<optgroup label={m.ls_group_arrivals()}>
							{#each arrivalTypes as t (t)}<option value={t}>{eventLabel(t)}</option>{/each}
						</optgroup>
						<optgroup label={m.ls_group_observations()}>
							{#each observationTypes as t (t)}<option value={t}>{eventLabel(t)}</option>{/each}
						</optgroup>
					</select>
					{#if typeHints[type]}<p class="hint">{typeHints[type]?.()}</p>{/if}
					{#if efe.type}<p class="field-error"><span aria-hidden="true">⚠</span> {efe.type}</p>{/if}
				</div>
				{#if !isObservation(type)}
					<Field
						label={m.ls_event_quantity_label()}
						name="quantity"
						type="number"
						min="1"
						max="10000"
						required
						bind:value={quantity}
						error={efe.quantity}
					/>
				{/if}
				<Field
					label={m.ls_occurred_at_label()}
					name="occurred_at"
					type="datetime-local"
					bind:value={occurredAt}
					hint={m.rec_timezone_note({ tz })}
					error={efe.occurred_at}
				/>
				<Field
					label={m.wc_notes_label()}
					name="note"
					optional={!isObservation(type)}
					required={isObservation(type)}
					multiline
					bind:value={note}
					error={efe.note}
				/>
				<div class="actions">
					<button type="submit" disabled={busy}>{busy ? m.saving() : m.ls_record()}</button>
				</div>
			</form>
		</section>
	{/if}

	<section class="card" aria-labelledby="history">
		<h2 id="history">{m.wc_history_title()}</h2>
		{#if events.length === 0}
			<p class="muted">{m.ls_history_empty()}</p>
		{:else}
			<ul class="list">
				{#each events as ev (ev.id)}
					<li class="item">
						<div class="row">
							<strong>{eventLabel(ev.type)}</strong>
							{#if ev.quantity_delta !== 0}
								<span class="qty">{deltaText(ev.quantity_delta)}</span>
							{/if}
							<time class="muted" datetime={ev.occurred_at}
								>{formatDateTime(ev.occurred_at, locale, tz)}</time
							>
						</div>
						{#if ev.note}<p class="notes">{ev.note}</p>{/if}
					</li>
				{/each}
			</ul>
			{#if cursor}
				<button type="button" class="secondary" onclick={more}>{m.load_more()}</button>
			{/if}
		{/if}
	</section>

	<ConfirmDialog
		bind:open={reconcileOpen}
		title={m.ls_reconcile_title()}
		confirmLabel={m.ls_reconcile()}
		{busy}
		onconfirm={reconcile}
	>
		<p>{m.ls_reconcile_body()}</p>
		{#if reconcileError && reconcileError.code !== 'validation_failed'}
			<Alert kind="error">{errorMessage(reconcileError)}</Alert>
		{/if}
		<Field
			label={m.ls_count_label()}
			name="count"
			type="number"
			min="0"
			max="100000"
			bind:value={count}
			error={rfe.count}
		/>
		<Field
			label={m.ls_occurred_at_label()}
			name="occurred_at"
			type="datetime-local"
			bind:value={reconcileAt}
			hint={m.rec_timezone_note({ tz })}
			error={rfe.occurred_at}
		/>
		<Field
			label={m.wc_notes_label()}
			name="note"
			optional
			bind:value={reconcileNote}
			error={rfe.note}
		/>
	</ConfirmDialog>
{/if}

<style>
	.meta {
		display: flex;
		flex-wrap: wrap;
		gap: 0.4rem;
	}
	.big {
		font-size: 2rem;
		font-weight: 700;
		margin: 0;
	}
	.notes {
		white-space: pre-line;
	}
	.list {
		list-style: none;
		margin: 0;
		padding: 0;
	}
	.item {
		border-top: 1px solid var(--border, #dde);
		padding: 0.6rem 0;
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
	.qty {
		font-weight: 600;
	}
	.item p {
		margin: 0.2rem 0;
	}
</style>
