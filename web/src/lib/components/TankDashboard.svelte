<!--
	Tank overview: what needs attention, what's next, livestock and equipment at a glance,
	and recent activity. Each block loads on its own, so one failing request doesn't hide
	the rest. Status is always given in words, never by colour alone.
-->
<script lang="ts">
	import { resolve } from '$app/paths';
	import {
		api,
		type DoseEvent,
		type Equipment,
		type LatestReading,
		type LivestockEntry,
		type MaintenancePlan,
		type Page,
		type WaterChange
	} from '#lib/api/client.js';
	import { doseText } from '#lib/dosing.js';
	import { formatDate, formatDateTime } from '#lib/format.js';
	import { groupByCategory, livestockCategoryLabel } from '#lib/livestock.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import { timelineText, type TimelineEvent } from '#lib/timeline.js';
	import { freshnessText, parameterLabel, targetStatusText, valueText } from '#lib/water-tests.js';

	let { tankId, timeZone }: { tankId: string; timeZone: string } = $props();

	const locale = getLocale();
	const base = $derived(`/tanks/${encodeURIComponent(tankId)}`);

	let wcOverdue = $state<WaterChange[] | null>(null);
	let wcNext = $state<WaterChange | null>(null);
	let doseAttention = $state<DoseEvent[] | null>(null);
	let doseNext = $state<DoseEvent[] | null>(null);
	let maintenance = $state<MaintenancePlan[] | null>(null);
	let livestock = $state<LivestockEntry[] | null>(null);
	let equipment = $state<Equipment[] | null>(null);
	let recent = $state<TimelineEvent[] | null>(null);
	let readings = $state<LatestReading[] | null>(null);
	let failed = $state(false);

	function list<T>(path: string, set: (items: T[]) => void) {
		api<Page<T>>('GET', path).then((r) => {
			if (r.ok) set(r.data.items);
			else failed = true;
		});
	}

	$effect(() => {
		const now = Date.now();
		const iso = (ms: number) => encodeURIComponent(new Date(ms).toISOString());
		list<WaterChange>(`${base}/water-changes?status=overdue&limit=50`, (i) => (wcOverdue = i));
		list<WaterChange>(
			`${base}/water-changes?status=scheduled&limit=50&to=${iso(now + 35 * 86_400_000)}`,
			(i) => (wcNext = i.at(-1) ?? null) // newest first: the last one is the soonest
		);
		list<DoseEvent>(
			`${base}/dosing-history?status=missed,unconfirmed&limit=50`,
			(i) => (doseAttention = i)
		);
		list<DoseEvent>(
			`${base}/dosing-history?status=scheduled&limit=200&from=${iso(now - 3_600_000)}&to=${iso(now + 24 * 3_600_000)}`,
			(i) => (doseNext = [...i].reverse().slice(0, 3))
		);
		list<MaintenancePlan>(`${base}/maintenance-plans`, (i) => (maintenance = i));
		list<LivestockEntry>(`${base}/livestock`, (i) => (livestock = i));
		list<Equipment>(`${base}/equipment`, (i) => (equipment = i));
		list<TimelineEvent>(`${base}/timeline?limit=8`, (i) => (recent = i));
		list<LatestReading>(`${base}/water-tests/latest`, (i) => (readings = i));
	});

	const maintenanceDue = $derived(
		(maintenance ?? []).filter((p) => p.active && (p.overdue || p.due_today))
	);
	const maintenanceNext = $derived(
		(maintenance ?? [])
			.filter((p) => p.active && !p.overdue && !p.due_today)
			.sort((a, b) => a.next_due_on.localeCompare(b.next_due_on))[0]
	);
	const measured = $derived((readings ?? []).filter((l) => l.reading));
	const offTarget = $derived(
		measured.filter(
			(l) => l.reading?.target_status === 'below' || l.reading?.target_status === 'above'
		)
	);
	const attentionCount = $derived(
		(wcOverdue?.length ?? 0) +
			(doseAttention?.length ?? 0) +
			maintenanceDue.length +
			offTarget.length
	);
	const allLoaded = $derived(
		wcOverdue !== null && doseAttention !== null && maintenance !== null && readings !== null
	);
</script>

{#if failed}
	<p class="muted" role="status">{m.db_partial()}</p>
{/if}

<section class="card" aria-labelledby="db-attention">
	<h2 id="db-attention">{m.db_attention_title()}</h2>
	{#if !allLoaded}
		<p role="status">{m.loading()}</p>
	{:else if attentionCount === 0}
		<p><span aria-hidden="true">✓</span> {m.db_all_clear()}</p>
	{:else}
		<ul class="tiles">
			{#if wcOverdue && wcOverdue.length > 0}
				<li>
					<a href={resolve('/(app)/tanks/[id]/water-changes', { id: tankId })}>
						<span aria-hidden="true">⚠</span>
						{m.db_wc_overdue({ count: String(wcOverdue.length) })}
					</a>
				</li>
			{/if}
			{#if doseAttention && doseAttention.length > 0}
				<li>
					<a href={resolve('/(app)/tanks/[id]/dosing', { id: tankId })}>
						<span aria-hidden="true">⚠</span>
						{m.db_dose_attention({ count: String(doseAttention.length) })}
					</a>
				</li>
			{/if}
			{#if maintenanceDue.length > 0}
				<li>
					<a href={resolve('/(app)/tanks/[id]/maintenance', { id: tankId })}>
						<span aria-hidden="true">⚠</span>
						{m.db_maintenance_due({ count: String(maintenanceDue.length) })}
					</a>
				</li>
			{/if}
			{#if offTarget.length > 0}
				<li>
					<a href={resolve('/(app)/tanks/[id]/water-tests', { id: tankId })}>
						<span aria-hidden="true">⚠</span>
						{m.db_off_target({
							count: String(offTarget.length),
							parameters: offTarget.map((l) => parameterLabel(l.parameter)).join(', ')
						})}
					</a>
				</li>
			{/if}
		</ul>
	{/if}
</section>

<section class="card" aria-labelledby="db-water">
	<h2 id="db-water">
		<a href={resolve('/(app)/tanks/[id]/water-tests', { id: tankId })}>{m.wp_title()}</a>
	</h2>
	{#if readings === null}
		<p role="status">{m.loading()}</p>
	{:else if measured.length === 0}
		<p class="muted">{m.wp_empty()}</p>
	{:else}
		<dl class="facts">
			{#each measured as l (l.parameter)}
				<dt>{parameterLabel(l.parameter)}</dt>
				<dd>
					{valueText(l.reading!.canonical_value, l.canonical_unit, locale)}
					{#if targetStatusText(l.reading!.target_status)}· {targetStatusText(
							l.reading!.target_status
						)}{/if}
					<span class="muted">· {l.stale ? '⚠ ' : ''}{freshnessText(l, locale)}</span>
				</dd>
			{/each}
		</dl>
	{/if}
</section>

<section class="card" aria-labelledby="db-next">
	<h2 id="db-next">{m.db_next_title()}</h2>
	<dl class="facts">
		<dt>{m.tank_section_water_changes()}</dt>
		<dd>
			{#if wcNext?.scheduled_for}
				<a href={resolve('/(app)/tanks/[id]/water-changes', { id: tankId })}
					>{formatDateTime(wcNext.scheduled_for, locale, timeZone)}</a
				>
				{#if wcNext.planned_volume_l}· {m.wc_planned_volume({
						volume: wcNext.planned_volume_l.replace('.', locale.startsWith('vi') ? ',' : '.')
					})}{/if}
			{:else}
				{m.db_none_planned()}
			{/if}
		</dd>
		<dt>{m.tank_section_dosing()}</dt>
		<dd>
			{#if doseNext && doseNext.length > 0}
				<ul class="plain">
					{#each doseNext as d (d.id)}
						<li>
							{d.product.name} · {doseText(d.planned_amount ?? '', d.amount_unit, locale)}
							{#if d.scheduled_for}· {formatDateTime(d.scheduled_for, locale, timeZone)}{/if}
						</li>
					{/each}
				</ul>
			{:else}
				{m.db_no_doses_today()}
			{/if}
		</dd>
		<dt>{m.tank_section_maintenance()}</dt>
		<dd>
			{#if maintenanceNext}
				{maintenanceNext.title} · {formatDate(maintenanceNext.next_due_on, locale)}
			{:else}
				{m.db_none_planned()}
			{/if}
		</dd>
	</dl>
</section>

<div class="pair">
	<section class="card" aria-labelledby="db-livestock">
		<h2 id="db-livestock">
			<a href={resolve('/(app)/tanks/[id]/livestock', { id: tankId })}>{m.ls_title()}</a>
		</h2>
		{#if livestock === null}
			<p role="status">{m.loading()}</p>
		{:else if livestock.length === 0}
			<p class="muted">{m.ls_empty()}</p>
		{:else}
			<ul class="plain">
				{#each groupByCategory(livestock) as [category, entries] (category)}
					<li>
						{livestockCategoryLabel(category)}: {entries.reduce(
							(n, e) => n + e.current_quantity,
							0
						)}
					</li>
				{/each}
			</ul>
		{/if}
	</section>
	<section class="card" aria-labelledby="db-equipment">
		<h2 id="db-equipment">
			<a href={resolve('/(app)/tanks/[id]/equipment', { id: tankId })}>{m.eq_title()}</a>
		</h2>
		{#if equipment === null}
			<p role="status">{m.loading()}</p>
		{:else}
			<p>{m.db_devices({ count: String(equipment.length) })}</p>
		{/if}
	</section>
</div>

<section class="card" aria-labelledby="db-recent">
	<h2 id="db-recent">{m.db_recent_title()}</h2>
	{#if recent === null}
		<p role="status">{m.loading()}</p>
	{:else if recent.length === 0}
		<p class="muted">{m.db_recent_empty()}</p>
	{:else}
		<ul class="plain recent">
			{#each recent as e (e.id)}
				<li>
					<time class="muted" datetime={e.occurred_at}
						>{formatDateTime(e.occurred_at, locale, timeZone)}</time
					>
					<span>{timelineText(e, locale)}</span>
				</li>
			{/each}
		</ul>
	{/if}
</section>

<style>
	.tiles {
		list-style: none;
		margin: 0;
		padding: 0;
		display: grid;
		gap: 0.5rem;
	}
	.plain {
		list-style: none;
		margin: 0;
		padding: 0;
	}
	.recent li {
		display: flex;
		flex-wrap: wrap;
		gap: 0.25rem 0.75rem;
		padding: 0.3rem 0;
		border-top: 1px solid var(--border, #dde);
	}
	.recent li:first-child {
		border-top: 0;
	}
	.pair {
		display: grid;
		gap: 1rem;
		grid-template-columns: repeat(auto-fit, minmax(14rem, 1fr));
	}
	h2 a {
		color: inherit;
	}
</style>
