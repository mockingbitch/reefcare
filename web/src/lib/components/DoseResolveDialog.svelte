<!--
	Records what happened to one scheduled, missed or unconfirmed dose. One dose at a time:
	there is deliberately no "mark all done", and nothing is ever confirmed automatically
	(docs/open-items.md DS5). A late dose starts with no outcome chosen and must say when it
	was given; the API never assumes now.
-->
<script lang="ts">
	import {
		api,
		type ApiError,
		type DoseEvent,
		type DosingPlan,
		type Page
	} from '#lib/api/client.js';
	import {
		doseText,
		isLate,
		nearbyDoses,
		nearbyWindowHours,
		quickConfirmAheadMs,
		unitLabel
	} from '#lib/dosing.js';
	import { errorMessage, fieldErrors } from '#lib/errors.js';
	import { formatDateTime } from '#lib/format.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import { decimalInput } from '#lib/tank-form.js';
	import { fromLocalInput, toLocalInput } from '#lib/water-changes.js';
	import Alert from './Alert.svelte';
	import ConfirmDialog from './ConfirmDialog.svelte';
	import Field from './Field.svelte';

	type Props = {
		open: boolean;
		dose: DoseEvent | null;
		tankId: string;
		/** The tank's plans, to size the "another dose nearby" window per plan. */
		plans: DosingPlan[];
		timeZone: string;
		onresolved: () => void;
	};

	let { open = $bindable(false), dose, tankId, plans, timeZone, onresolved }: Props = $props();

	const locale = getLocale();
	const uid = $props.id();
	type Outcome = 'completed' | 'partial' | 'skipped' | 'failed';
	type When = 'scheduled' | 'now' | 'other';

	let outcome = $state<Outcome | ''>('');
	let amount = $state('');
	let when = $state<When | ''>('');
	let other = $state('');
	let reason = $state('');
	let notes = $state('');
	let confirmOver = $state(false);
	let outcomeMissing = $state(false);
	let busy = $state(false);
	let error = $state<ApiError | null>(null);
	const fe = $derived(fieldErrors(error));
	const overPlanned = $derived(
		(error?.details ?? []).some((d) => d.code === 'confirm_over_planned')
	);

	const late = $derived(dose ? isLate(dose) : false);
	const given = $derived(outcome === 'completed' || outcome === 'partial');
	const early = $derived(
		!!dose &&
			!late &&
			!!dose.scheduled_for &&
			Date.parse(dose.scheduled_for) - Date.now() > quickConfirmAheadMs
	);

	// Reset the form whenever a different dose is opened.
	let openedFor = '';
	$effect(() => {
		if (!open || !dose || openedFor === dose.id) return;
		openedFor = dose.id;
		const isLateDose = isLate(dose);
		outcome = isLateDose ? '' : 'completed';
		amount = isLateDose ? '' : (dose.planned_amount ?? '');
		when = isLateDose ? '' : 'now';
		other = toLocalInput(dose.scheduled_for ?? new Date().toISOString(), timeZone);
		reason = '';
		notes = '';
		confirmOver = false;
		outcomeMissing = false;
		error = null;
	});
	$effect(() => {
		if (!open) openedFor = '';
	});

	// The planned amount is only ever a default for "given as planned"; a partial or failed
	// dose must never silently record the full plan as having gone in.
	function chooseOutcome(next: Outcome) {
		outcome = next;
		outcomeMissing = false;
		amount = next === 'completed' ? (dose?.planned_amount ?? '') : '';
	}

	const executedAt = $derived.by(() => {
		if (!given || !dose) return undefined;
		if (when === 'scheduled') return dose.scheduled_for ?? undefined;
		if (when === 'now') return late ? new Date().toISOString() : undefined; // API default: now
		if (when === 'other') return fromLocalInput(other, timeZone) || undefined;
		return undefined;
	});

	// Doses of the same product around the chosen time, fetched for exactly that window so
	// busy multi-part or pump tanks can't push them out of a page-sized list.
	let nearby = $state<DoseEvent[]>([]);
	let seq = 0;
	$effect(() => {
		const d = dose;
		const at = executedAt ?? new Date().toISOString();
		const hours = nearbyWindowHours(plans.find((p) => p.id === d?.plan_id));
		const mine = ++seq;
		if (!d || !given || !open || hours === null) {
			nearby = [];
			return;
		}
		const t = Date.parse(at);
		const query = new URLSearchParams({
			from: new Date(t - hours * 3_600_000).toISOString(),
			to: new Date(t + hours * 3_600_000).toISOString(),
			limit: '200'
		}).toString();
		const timer = setTimeout(async () => {
			const r = await api<Page<DoseEvent>>(
				'GET',
				`/tanks/${encodeURIComponent(tankId)}/dosing-history?${query}`
			);
			if (mine !== seq) return;
			nearby = r.ok ? nearbyDoses(at, d.product.id, r.data.items, d.id, hours) : [];
		}, 300);
		return () => clearTimeout(timer);
	});
	const nearbyUnconfirmed = $derived(nearby.some((e) => e.status === 'unconfirmed'));

	async function submit() {
		if (!dose) return;
		if (outcome === '') {
			outcomeMissing = true;
			return;
		}
		busy = true;
		error = null;
		const body: Record<string, unknown> = { status: outcome, notes: notes.trim() || null };
		if (given) {
			if (amount.trim() !== '') body.actual_amount = decimalInput(amount);
			if (executedAt) body.executed_at = executedAt;
			if (confirmOver) body.confirm_over_planned = true;
		} else {
			body.reason = reason.trim() || null;
			if (outcome === 'failed' && amount.trim() !== '') body.actual_amount = decimalInput(amount);
		}
		const r = await api<DoseEvent>('POST', `/dosing-events/${dose.id}/resolve`, body);
		busy = false;
		if (!r.ok) {
			error = r.error;
			return;
		}
		open = false;
		onresolved();
	}

	const title = $derived(
		!dose
			? ''
			: late
				? m.dose_record_what_happened()
				: m.dose_confirm_title({
						dose: doseText(dose.planned_amount ?? '', dose.amount_unit, locale),
						product: dose.product.name
					})
	);

	const outcomes: [Outcome, () => string][] = [
		['completed', m.dose_outcome_completed],
		['partial', m.dose_outcome_partial],
		['skipped', m.dose_outcome_skipped],
		['failed', m.dose_outcome_failed]
	];
</script>

<ConfirmDialog bind:open {title} confirmLabel={m.dose_record()} {busy} onconfirm={submit}>
	{#if dose}
		<p>
			<strong>{dose.product.name}</strong>
			{#if dose.planned_amount}· {doseText(dose.planned_amount, dose.amount_unit, locale)}{/if}
		</p>
		{#if dose.scheduled_for}
			<p class="muted">
				{m.dose_scheduled_for({ when: formatDateTime(dose.scheduled_for, locale, timeZone) })}
			</p>
		{/if}
		{#if dose.status === 'missed'}
			<Alert kind="info">{m.dose_missed_note()}</Alert>
		{:else if dose.status === 'unconfirmed'}
			<Alert kind="info">{m.dose_unconfirmed_note()}</Alert>
		{:else if early && dose.scheduled_for}
			<Alert kind="warning"
				>{m.dose_early_note({
					when: formatDateTime(dose.scheduled_for, locale, timeZone)
				})}</Alert
			>
		{/if}
		{#if error && error.code !== 'validation_failed'}
			<Alert kind="error">{errorMessage(error)}</Alert>
		{/if}

		<fieldset aria-describedby={outcomeMissing || fe.status ? `${uid}-outcome-error` : undefined}>
			<legend>{m.dose_outcome_label()}</legend>
			{#each outcomes as [value, label] (value)}
				<label class="choice">
					<input
						type="radio"
						name="outcome"
						{value}
						checked={outcome === value}
						onchange={() => chooseOutcome(value)}
					/>
					{label()}
				</label>
			{/each}
			{#if outcomeMissing}
				<p id="{uid}-outcome-error" class="field-error">
					<span aria-hidden="true">⚠</span>
					{m.dose_outcome_required()}
				</p>
			{:else if fe.status}
				<p id="{uid}-outcome-error" class="field-error">
					<span aria-hidden="true">⚠</span>
					{fe.status}
				</p>
			{/if}
		</fieldset>

		{#if given || outcome === 'failed'}
			<Field
				label={outcome === 'failed'
					? m.dose_failed_amount_label({ unit: unitLabel(dose.amount_unit) })
					: m.dose_actual_amount_label({ unit: unitLabel(dose.amount_unit) })}
				name="actual_amount"
				inputmode="decimal"
				optional={outcome !== 'partial'}
				bind:value={amount}
				error={fe.actual_amount}
			/>
		{/if}
		{#if overPlanned}
			<label class="choice">
				<input type="checkbox" name="confirm_over_planned" bind:checked={confirmOver} />
				{m.dose_confirm_over()}
			</label>
		{/if}

		{#if given}
			<fieldset>
				<legend>{late ? m.dose_when_late_label() : m.dose_when_label()}</legend>
				{#if dose.scheduled_for}
					<label class="choice">
						<input type="radio" name="when" value="scheduled" bind:group={when} />
						{m.dose_when_scheduled()}
					</label>
				{/if}
				<label class="choice">
					<input type="radio" name="when" value="now" bind:group={when} />
					{m.dose_when_now()}
				</label>
				<label class="choice">
					<input type="radio" name="when" value="other" bind:group={when} />
					{m.dose_when_other()}
				</label>
				{#if when === 'other'}
					<Field
						label={m.dose_when_other()}
						name="executed_at"
						type="datetime-local"
						bind:value={other}
						hint={m.rec_timezone_note({ tz: timeZone })}
					/>
				{/if}
				{#if fe.executed_at}
					<p class="field-error"><span aria-hidden="true">⚠</span> {fe.executed_at}</p>
				{/if}
			</fieldset>
			{#if nearby.length > 0}
				{@const at = nearby[0].executed_at ?? nearby[0].scheduled_for ?? ''}
				<Alert kind="warning">
					{nearbyUnconfirmed
						? m.dose_nearby_unconfirmed_warning({ when: formatDateTime(at, locale, timeZone) })
						: m.dose_nearby_warning({ when: formatDateTime(at, locale, timeZone) })}
				</Alert>
			{/if}
		{:else if outcome !== ''}
			<Field
				label={m.wc_skip_reason_label()}
				name="reason"
				required
				bind:value={reason}
				error={fe.reason}
			/>
		{/if}

		<Field
			label={m.wc_notes_label()}
			name="notes"
			optional
			multiline
			bind:value={notes}
			error={fe.notes}
		/>
	{/if}
</ConfirmDialog>

<style>
	fieldset {
		border: 0;
		padding: 0;
		margin: 0.75rem 0;
	}
	legend {
		font-weight: 600;
		margin-bottom: 0.25rem;
	}
	.choice {
		display: flex;
		align-items: center;
		gap: 0.4rem;
		font-weight: normal;
		margin: 0.2rem 0;
	}
</style>
