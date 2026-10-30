<!--
	Recurrence controls (weekly / daily / monthly, one time of day) with a live preview of
	the next dates. The preview comes from GET /recurrence/preview, so it applies exactly the
	rules the server uses when the schedule is saved (open item W6).
-->
<script lang="ts">
	import { api, type RecurrencePreview } from '#lib/api/client.js';
	import { formatDateTime } from '#lib/format.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import {
		maxMonthDay,
		monthDayCapped,
		toRRule,
		weekdayLabel,
		weekdays,
		type RecurrenceForm
	} from '#lib/recurrence.js';
	import Field from './Field.svelte';
	import SelectField from './SelectField.svelte';

	type Props = {
		form: RecurrenceForm;
		startsOn: string;
		endsOn: string;
		timeZone: string;
		/** Server field error for `rrule`, if any. */
		error?: string;
	};

	let { form = $bindable(), startsOn, endsOn, timeZone, error }: Props = $props();

	const locale = getLocale();
	const legendId = $props.id();

	let preview = $state<RecurrencePreview | null>(null);
	let previewInvalid = $state(false);

	const intervalLabel = $derived(
		form.freq === 'DAILY'
			? m.rec_interval_days()
			: form.freq === 'WEEKLY'
				? m.rec_interval_weeks()
				: m.rec_interval_months()
	);

	// Debounced preview: refetch 300 ms after the last change; stale answers are ignored.
	let seq = 0;
	$effect(() => {
		const params: Record<string, string> = {
			rrule: toRRule(form),
			timezone: timeZone,
			starts_on: startsOn
		};
		if (endsOn) params.ends_on = endsOn;
		const query = new URLSearchParams(params).toString();
		const mine = ++seq;
		const timer = setTimeout(async () => {
			if (!startsOn) {
				preview = null;
				previewInvalid = true;
				return;
			}
			const r = await api<RecurrencePreview>('GET', `/recurrence/preview?${query}`);
			if (mine !== seq) return;
			preview = r.ok ? r.data : null;
			previewInvalid = !r.ok;
		}, 300);
		return () => clearTimeout(timer);
	});

	function toggleDay(day: (typeof weekdays)[number], checked: boolean) {
		form.days = checked ? [...form.days, day] : form.days.filter((d) => d !== day);
	}
</script>

<fieldset class="recurrence" aria-describedby={error ? `${legendId}-error` : undefined}>
	<legend id={legendId}>{m.rec_legend()}</legend>

	<SelectField
		label={m.rec_freq_label()}
		name="freq"
		bind:value={form.freq}
		options={[
			{ value: 'WEEKLY', label: m.rec_freq_weekly() },
			{ value: 'DAILY', label: m.rec_freq_daily() },
			{ value: 'MONTHLY', label: m.rec_freq_monthly() }
		]}
	/>

	<Field
		label={intervalLabel}
		name="interval"
		type="number"
		min="1"
		max="365"
		bind:value={form.interval}
	/>

	{#if form.freq === 'WEEKLY'}
		<fieldset class="days">
			<legend>{m.rec_days_label()}</legend>
			{#each weekdays as day (day)}
				<label class="check">
					<input
						type="checkbox"
						name="byday"
						value={day}
						checked={form.days.includes(day)}
						onchange={(e) => toggleDay(day, e.currentTarget.checked)}
					/>
					{weekdayLabel(day)}
				</label>
			{/each}
		</fieldset>
	{:else if form.freq === 'MONTHLY'}
		<Field
			label={m.rec_monthday_label()}
			name="bymonthday"
			type="number"
			min="1"
			max={String(maxMonthDay)}
			bind:value={form.monthDay}
			hint={monthDayCapped(startsOn)
				? m.rec_monthday_capped({ day: startsOn.split('-')[2] })
				: undefined}
		/>
	{/if}

	<Field
		label={m.rec_time_label()}
		name="time"
		type="time"
		bind:value={form.time}
		hint={m.rec_timezone_note({ tz: timeZone })}
	/>

	{#if error}
		<p id="{legendId}-error" class="field-error"><span aria-hidden="true">⚠</span> {error}</p>
	{/if}

	<div class="preview" aria-live="polite">
		<h3>{m.rec_preview_title()}</h3>
		{#if previewInvalid}
			<p class="muted">{m.rec_preview_invalid()}</p>
		{:else if preview && preview.next_occurrences.length === 0}
			<p class="muted">{m.rec_preview_none()}</p>
		{:else if preview}
			<ol>
				{#each preview.next_occurrences as at (at)}
					<li><time datetime={at}>{formatDateTime(at, locale, timeZone)}</time></li>
				{/each}
			</ol>
		{/if}
	</div>
</fieldset>

<style>
	.recurrence {
		border: 1px solid var(--border, #ccd);
		border-radius: 0.5rem;
		padding: 0.75rem 1rem;
	}
	.days {
		border: 0;
		padding: 0;
		display: flex;
		flex-wrap: wrap;
		gap: 0.25rem 1rem;
	}
	.check {
		display: inline-flex;
		align-items: center;
		gap: 0.35rem;
		font-weight: normal;
	}
	.preview h3 {
		font-size: 1rem;
		margin: 0.75rem 0 0.25rem;
	}
	.preview ol {
		margin: 0;
		padding-left: 1.25rem;
	}
</style>
