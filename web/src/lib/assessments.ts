// Renders assessment texts (api/openapi.yaml I18nText): every finding sentence is a key +
// params from internal/intelligence. Unknown keys render generically so a newer rule set never
// breaks the page. Params are inserted as text by Svelte (never as HTML): some, like
// knowledge safety warnings, are editor-written.
import type { Finding, I18nText } from '#lib/api/client.js';
import { formatDecimal } from '#lib/format.js';
import { m } from '#lib/paraglide/messages.js';
import { parameterLabel, unitLabel } from '#lib/water-tests.js';

type Messages = Record<string, ((p?: Record<string, string>) => string) | undefined>;
const messages = m as unknown as Messages;

// Params holding canonical decimal values (formatted for the locale).
const numeric = new Set([
	'value',
	'from',
	'to',
	'a',
	'b',
	'bound',
	'limit',
	'min',
	'max',
	'range',
	'projected',
	'slope',
	'threshold',
	'days'
]);

const kinds: Record<string, () => string> = {
	readings: m.ia_kind_readings,
	dosing_events: m.ia_kind_dosing_events,
	water_changes: m.ia_kind_water_changes,
	equipment_changes: m.ia_kind_equipment_changes,
	livestock_events: m.ia_kind_livestock_events
};

/** The message function name of a key: "nh.summary.urgent" → "ia_nh_summary_urgent". */
export const messageName = (key: string) => 'ia_' + key.replace(/\./g, '_');

export function assessmentText(t: I18nText, locale: string): string {
	const fn = messages[messageName(t.key)];
	if (!fn) return m.ia_unknown();
	const params: Record<string, string> = {};
	for (const [k, v] of Object.entries(t.params ?? {})) {
		if (k === 'parameter') params[k] = parameterLabel(v);
		else if (k === 'unit') params[k] = unitLabel(v);
		else if (k === 'kind') params[k] = kinds[v]?.() ?? v;
		else if (numeric.has(k) && /^-?\d+(\.\d+)?$/.test(v)) params[k] = formatDecimal(v, locale);
		else params[k] = v;
	}
	return fn(params);
}

export function severityLabel(s: string): string {
	return (
		{
			info: m.ia_sev_info,
			notice: m.ia_sev_notice,
			warning: m.ia_sev_warning,
			urgent: m.ia_sev_urgent
		}[s] ?? m.ia_sev_info
	)();
}

export function confidenceLabel(c: string): string {
	return (
		{ low: m.ia_conf_low, moderate: m.ia_conf_moderate, high: m.ia_conf_high }[c] ??
		m.ia_conf_not_assessed
	)();
}

export function triggerLabel(t: string): string {
	return (
		{
			manual: m.ia_trigger_manual,
			water_test: m.ia_trigger_water_test,
			scheduled: m.ia_trigger_scheduled
		}[t] ?? m.ia_trigger_manual
	)();
}

/** Severity icons: status is never shown by color alone. */
export const severityIcon = (s: string) =>
	({ urgent: '⛔', warning: '⚠', notice: 'ℹ', info: '·' })[s] ?? '·';

/** Safety warnings with urgent ones first (they are never hidden). */
export function orderedWarnings(f: Finding) {
	return [...f.safety_warnings].sort((a, b) => Number(b.urgent) - Number(a.urgent));
}

/** How many distinct facts a finding's observations cite. */
export function evidenceCount(f: Finding): number {
	return new Set(f.observations.flatMap((o) => o.evidence.map((e) => `${e.kind}:${e.id}`))).size;
}
