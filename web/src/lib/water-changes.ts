// Presentation helpers for water changes. The API computes every estimate; these only
// format it with its caveats (api/openapi.yaml WaterChangeEstimate).
import type { WaterChangeEstimate } from '#lib/api/client.js';
import { m } from '#lib/paraglide/messages.js';
import { formatDecimal } from '#lib/format.js';

/**
 * The exchanged share as a percentage with one decimal, shown as "< 0.1 %" when a non-zero
 * volume rounds to zero (the API contract asks for this). Exact decimal arithmetic on the
 * string: the API sends 3 decimals (e.g. "0.139" → "13.9").
 */
export function percentOfSystem(fraction: string, locale: string): string {
	const [int = '0', frac = ''] = fraction.split('.');
	const digits = (frac + '000').slice(0, 3);
	const tenths = Number(int) * 1000 + Number(digits); // in 0.1 % units
	if (tenths === 0) {
		return /[1-9]/.test(fraction) ? m.percent_below_tenth() : m.percent_value({ value: '0' });
	}
	const whole = Math.trunc(tenths / 10);
	const rest = tenths % 10;
	const value = rest === 0 ? String(whole) : `${whole}.${rest}`;
	return m.percent_value({ value: formatDecimal(value, locale) });
}

const assumptionLabels: Record<string, () => string> = {
	fraction_of_total_system: m.wc_assumption_total_system,
	complete_mixing: m.wc_assumption_complete_mixing,
	system_volume_as_entered: m.wc_assumption_volume_as_entered
};

const flagLabels: Record<string, () => string> = {
	exceeds_system_volume: m.wc_flag_exceeds_system,
	large_change: m.wc_flag_large_change
};

export const assumptionLabel = (a: string) => assumptionLabels[a]?.() ?? a;
export const flagLabel = (f: string) => flagLabels[f]?.() ?? f;

export function estimateSummary(e: WaterChangeEstimate, locale: string): string {
	return m.wc_estimate_summary({
		percent: percentOfSystem(e.fraction_of_system, locale),
		volume: formatDecimal(e.system_volume_l, locale)
	});
}

/** `datetime-local` value (YYYY-MM-DDTHH:MM) for an instant in a time zone. */
export function toLocalInput(iso: string, timeZone: string): string {
	const parts = new Intl.DateTimeFormat('en-CA', {
		timeZone,
		year: 'numeric',
		month: '2-digit',
		day: '2-digit',
		hour: '2-digit',
		minute: '2-digit',
		hourCycle: 'h23'
	}).formatToParts(new Date(iso));
	const get = (t: string) => parts.find((p) => p.type === t)?.value ?? '00';
	return `${get('year')}-${get('month')}-${get('day')}T${get('hour')}:${get('minute')}`;
}

/**
 * Converts a `datetime-local` value entered in `timeZone` to an RFC 3339 UTC instant,
 * independent of the browser's own zone. Returns '' for an incomplete value.
 */
export function fromLocalInput(value: string, timeZone: string): string {
	const match = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})$/.exec(value);
	if (!match) return '';
	const [y, mo, d, h, mi] = match.slice(1).map(Number);
	const asUTC = Date.UTC(y, mo - 1, d, h, mi);
	// Offset of the zone at that wall-clock time; one correction pass handles DST shifts.
	let instant = asUTC - offsetMs(asUTC, timeZone);
	instant = asUTC - offsetMs(instant, timeZone);
	return new Date(instant).toISOString().replace('.000Z', 'Z');
}

function offsetMs(instant: number, timeZone: string): number {
	const local = toLocalInput(new Date(instant).toISOString(), timeZone);
	const [date, time] = local.split('T');
	const [y, mo, d] = date.split('-').map(Number);
	const [h, mi] = time.split(':').map(Number);
	const truncated = instant - (instant % 60_000);
	return Date.UTC(y, mo - 1, d, h, mi) - truncated;
}
