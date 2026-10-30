<!--
	One parameter over time (one y-axis per chart; several parameters are small multiples).
	Target versions are shaded bands, dosing/water-change/equipment events are dashed markers
	with a letter, estimates are hollow points. A crosshair tooltip follows the pointer or the
	arrow keys, and a text summary plus a data table give the same information without the chart.
-->
<script lang="ts">
	import type { Trends } from '#lib/api/client.js';
	import { formatDateTime } from '#lib/format.js';
	import { m } from '#lib/paraglide/messages.js';
	import { annotationLetter, annotationLabel } from '#lib/timeline.js';
	import {
		niceTicks,
		parameterLabel,
		scale,
		unitLabel,
		valueDomain,
		valueText
	} from '#lib/water-tests.js';

	type Series = Trends['series'][number];
	type Annotation = Trends['annotations'][number];

	let {
		series,
		annotations,
		from,
		to,
		locale,
		timeZone
	}: {
		series: Series;
		annotations: Annotation[];
		from: string;
		to: string;
		locale: string;
		timeZone: string;
	} = $props();

	const W = 640;
	const H = 240;
	const pad = { top: 22, right: 16, bottom: 28, left: 48 };
	const id = $props.id();

	const t0 = $derived(Date.parse(from));
	const t1 = $derived(Date.parse(to));
	const values = $derived(series.points.map((p) => Number(p.v)));
	const yTicks = $derived(niceTicks(...valueDomain(values, series.targets)));
	const x = $derived(scale(t0, t1, pad.left, W - pad.right));
	const y = $derived(scale(yTicks[0] ?? 0, yTicks.at(-1) ?? 1, H - pad.bottom, pad.top));
	const pts = $derived(
		series.points.map((p) => ({ ...p, cx: x(Date.parse(p.t)), cy: y(Number(p.v)) }))
	);
	const path = $derived(
		pts.map((p, i) => `${i ? 'L' : 'M'}${p.cx.toFixed(1)},${p.cy.toFixed(1)}`).join('')
	);
	const bands = $derived(
		series.targets
			.filter((t) => t.min || t.max)
			.map((t) => {
				const a = Math.max(t0, Date.parse(t.valid_from));
				const b = Math.min(t1, t.valid_to ? Date.parse(t.valid_to) : t1);
				const top = y(t.max ? Number(t.max) : (yTicks.at(-1) ?? 1));
				const bottom = y(t.min ? Number(t.min) : (yTicks[0] ?? 0));
				return { x: x(a), w: Math.max(0, x(b) - x(a)), y: top, h: Math.max(0, bottom - top) };
			})
			.filter((b) => b.w > 0)
	);
	const xTicks = $derived(
		Array.from({ length: 5 }, (_, i) => t0 + ((t1 - t0) * i) / 4).map((t) => ({
			x: x(t),
			label: new Intl.DateTimeFormat(locale, { day: 'numeric', month: 'short', timeZone }).format(t)
		}))
	);
	const marks = $derived(
		annotations.map((a) => ({ ...a, ax: x(Date.parse(a.t)), letter: annotationLetter(a.category) }))
	);
	const unit = $derived(unitLabel(series.unit));

	let active = $state<number | null>(null);
	const current = $derived(active === null ? null : pts[active]);

	function nearest(clientX: number, svg: SVGSVGElement) {
		if (pts.length === 0) return;
		const r = svg.getBoundingClientRect();
		const vx = ((clientX - r.left) / r.width) * W;
		let best = 0;
		for (let i = 1; i < pts.length; i++) {
			if (Math.abs(pts[i].cx - vx) < Math.abs(pts[best].cx - vx)) best = i;
		}
		active = best;
	}

	function onkey(e: KeyboardEvent) {
		if (pts.length === 0) return;
		if (e.key === 'ArrowRight' || e.key === 'ArrowLeft') {
			e.preventDefault();
			const d = e.key === 'ArrowRight' ? 1 : -1;
			active =
				active === null
					? d > 0
						? 0
						: pts.length - 1
					: Math.min(pts.length - 1, Math.max(0, active + d));
		} else if (e.key === 'Escape') {
			active = null;
		}
	}

	const summary = $derived.by(() => {
		if (pts.length === 0) return m.wp_chart_empty();
		const last = series.points.at(-1)!;
		const sorted = [...series.points].sort((a, b) => Number(a.v) - Number(b.v));
		return m.wp_chart_summary({
			count: String(pts.length),
			latest: valueText(last.v, series.unit, locale),
			min: valueText(sorted[0].v, series.unit, locale),
			max: valueText(sorted.at(-1)!.v, series.unit, locale)
		});
	});
	const fmt = (v: string) => valueText(v, series.unit, locale);
</script>

<figure class="chart" aria-labelledby="{id}-title">
	<figcaption>
		<h3 id="{id}-title">{parameterLabel(series.parameter)}{unit ? ` · ${unit}` : ''}</h3>
		<p class="muted" id="{id}-summary">{summary}</p>
	</figcaption>
	<div class="plot">
		<!-- The image is also keyboard-explorable (arrow keys move the tooltip); the table below is the full alternative. -->
		<!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
		<svg
			viewBox="0 0 {W} {H}"
			role="img"
			aria-labelledby="{id}-title"
			aria-describedby="{id}-summary {id}-hint"
			tabindex={pts.length ? 0 : undefined}
			onpointermove={(e) => nearest(e.clientX, e.currentTarget)}
			onpointerleave={() => (active = null)}
			onkeydown={onkey}
			onblur={() => (active = null)}
		>
			{#each bands as b, i (i)}
				<rect class="band" x={b.x} y={b.y} width={b.w} height={b.h} />
			{/each}
			{#each yTicks as v (v)}
				<line class="grid" x1={pad.left} x2={W - pad.right} y1={y(v)} y2={y(v)} />
				<text class="tick" x={pad.left - 6} y={y(v)} text-anchor="end" dominant-baseline="middle"
					>{v.toLocaleString(locale)}</text
				>
			{/each}
			{#each xTicks as t, i (i)}
				<text
					class="tick"
					x={t.x}
					y={H - 8}
					text-anchor={i === 0 ? 'start' : i === 4 ? 'end' : 'middle'}>{t.label}</text
				>
			{/each}
			{#each marks as a, i (i)}
				<line class="event" x1={a.ax} x2={a.ax} y1={pad.top - 4} y2={H - pad.bottom} />
				<text class="event-letter" x={a.ax} y={pad.top - 8} text-anchor="middle">{a.letter}</text>
			{/each}
			{#if pts.length > 1}<path class="line" d={path} />{/if}
			{#if current}
				<line class="crosshair" x1={current.cx} x2={current.cx} y1={pad.top} y2={H - pad.bottom} />
			{/if}
			{#each pts as p, i (p.reading_id)}
				<circle
					class="point"
					class:estimate={p.estimate}
					class:active={i === active}
					cx={p.cx}
					cy={p.cy}
					r={i === active ? 6 : 4}
				/>
			{/each}
		</svg>
		{#if current}
			<div
				class="tooltip"
				role="status"
				style:left="{(current.cx / W) * 100}%"
				style:top="{(current.cy / H) * 100}%"
				class:flip={current.cx > W * 0.65}
			>
				<strong>{fmt(current.v)}</strong>{#if current.estimate}
					· {m.wp_estimate()}{/if}<br />
				<span>{formatDateTime(current.t, locale, timeZone)}</span>
			</div>
		{/if}
	</div>
	<p class="legend muted" id="{id}-hint">
		{#if bands.length}<span
				><span class="swatch" aria-hidden="true"></span> {m.wp_chart_target_band()}</span
			>{/if}
		{#if pts.some((p) => p.estimate)}<span>○ {m.wp_estimate()}</span>{/if}
		{#each [...new Set(marks.map((a) => a.category))] as c (c)}
			<span><b>{annotationLetter(c)}</b> {annotationLabel(c)}</span>
		{/each}
		{#if pts.length}<span>{m.wp_chart_keys_hint()}</span>{/if}
	</p>
	{#if series.points.length}
		<details>
			<summary>{m.wp_chart_table()}</summary>
			<table>
				<thead>
					<tr><th scope="col">{m.wp_measured_at()}</th><th scope="col">{m.wp_value()}</th></tr>
				</thead>
				<tbody>
					{#each [...series.points].reverse() as p (p.reading_id)}
						<tr>
							<td><time datetime={p.t}>{formatDateTime(p.t, locale, timeZone)}</time></td>
							<td>{fmt(p.v)}{p.estimate ? ` (${m.wp_estimate()})` : ''}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</details>
	{/if}
</figure>

<style>
	.chart {
		margin: 0 0 1.5rem;
	}
	h3 {
		margin: 0;
		font-size: 1.05rem;
	}
	figcaption p {
		margin: 0.2rem 0 0.5rem;
	}
	.plot {
		position: relative;
	}
	svg {
		display: block;
		width: 100%;
		height: auto;
		touch-action: pan-y;
	}
	svg:focus-visible {
		outline: 3px solid var(--focus);
		outline-offset: 2px;
	}
	.band {
		fill: var(--accent);
		opacity: 0.12;
	}
	.grid {
		stroke: var(--border);
		stroke-width: 1;
		opacity: 0.6;
	}
	.tick {
		fill: var(--muted);
		font-size: 12px;
	}
	.event {
		stroke: var(--muted);
		stroke-dasharray: 3 3;
		stroke-width: 1;
	}
	.event-letter {
		fill: var(--muted);
		font-size: 11px;
		font-weight: 600;
	}
	.line {
		fill: none;
		stroke: var(--accent);
		stroke-width: 2;
		stroke-linejoin: round;
	}
	.crosshair {
		stroke: var(--text);
		stroke-width: 1;
		opacity: 0.4;
	}
	.point {
		fill: var(--accent);
		stroke: var(--surface);
		stroke-width: 2;
	}
	.point.estimate {
		fill: var(--surface);
		stroke: var(--accent);
	}
	.tooltip {
		position: absolute;
		transform: translate(12px, -50%);
		background: var(--surface);
		color: var(--text);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		padding: 0.35rem 0.55rem;
		font-size: 0.85rem;
		pointer-events: none;
		white-space: nowrap;
		box-shadow: 0 2px 6px rgb(0 0 0 / 0.15);
	}
	.tooltip.flip {
		transform: translate(calc(-100% - 12px), -50%);
	}
	.legend {
		display: flex;
		flex-wrap: wrap;
		gap: 0.25rem 1rem;
		font-size: 0.85rem;
		margin: 0.25rem 0;
	}
	.swatch {
		display: inline-block;
		width: 0.9em;
		height: 0.9em;
		vertical-align: -0.1em;
		background: var(--accent);
		opacity: 0.25;
		border-radius: 2px;
	}
	table {
		border-collapse: collapse;
		margin-top: 0.5rem;
	}
	th,
	td {
		text-align: left;
		padding: 0.2rem 0.75rem 0.2rem 0;
		border-bottom: 1px solid var(--border);
	}
</style>
