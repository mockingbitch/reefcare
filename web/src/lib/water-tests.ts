// Water-test helpers. The API owns conversions, bounds and target comparison; these helpers
// only build requests and render what it returns. Units are shown exactly as entered.
import {
	api,
	type ApiError,
	type LatestReading,
	type Page,
	type ParameterType,
	type Reading,
	type Target,
	type TargetInput,
	type WaterTestCreate
} from '#lib/api/client.js';
import { formatDecimal } from '#lib/format.js';
import { m } from '#lib/paraglide/messages.js';
import { decimalInput } from '#lib/tank-form.js';

let catalogCache: ParameterType[] | null = null;

/** The parameter catalog (static per release), fetched once per page load. */
export async function loadCatalog(): Promise<
	{ ok: true; data: ParameterType[] } | { ok: false; error: ApiError }
> {
	if (catalogCache) return { ok: true, data: catalogCache };
	const r = await api<Page<ParameterType>>('GET', '/parameter-types');
	if (!r.ok) return r;
	catalogCache = r.data.items;
	return { ok: true, data: catalogCache };
}

const parameterLabels: Record<string, () => string> = {
	alkalinity: m.wp_p_alkalinity,
	calcium: m.wp_p_calcium,
	magnesium: m.wp_p_magnesium,
	ph: m.wp_p_ph,
	temperature: m.wp_p_temperature,
	salinity: m.wp_p_salinity,
	nitrate: m.wp_p_nitrate,
	phosphate: m.wp_p_phosphate,
	ammonia: m.wp_p_ammonia,
	nitrite: m.wp_p_nitrite
};
export const parameterLabel = (code: string) => parameterLabels[code]?.() ?? code;

const unitText: Record<string, string> = { C: '°C', F: '°F', pH: '' };
/** Unit as shown next to a value ("8,2 dKH", "25 °C", "pH 8,1" is shown as "8,1"). */
export const unitLabel = (code: string) => unitText[code] ?? code;

export function valueText(value: string, unit: string, locale: string): string {
	const u = unitLabel(unit);
	return u ? `${formatDecimal(value, locale)} ${u}` : formatDecimal(value, locale);
}

export const methods = [
	'titration',
	'colorimetric',
	'photometer',
	'refractometer',
	'hydrometer',
	'probe',
	'icp',
	'other'
] as const;
const methodLabels: Record<string, () => string> = {
	titration: m.wp_m_titration,
	colorimetric: m.wp_m_colorimetric,
	photometer: m.wp_m_photometer,
	refractometer: m.wp_m_refractometer,
	hydrometer: m.wp_m_hydrometer,
	probe: m.wp_m_probe,
	icp: m.wp_m_icp,
	other: m.wp_m_other
};
export const methodLabel = (code: string) => methodLabels[code]?.() ?? code;

/** Target status in words with a non-colour marker (frontend rules). */
export function targetStatusText(status: Reading['target_status']): string | null {
	switch (status) {
		case 'below':
			return `↓ ${m.wp_status_below()}`;
		case 'above':
			return `↑ ${m.wp_status_above()}`;
		case 'within':
			return `✓ ${m.wp_status_within()}`;
		default:
			return null;
	}
}

const warningLabels: Record<string, () => string> = {
	looks_like_specific_gravity: m.wp_warn_sg,
	unusually_high: m.wp_warn_high
};
export const warningLabel = (code: string) => warningLabels[code]?.() ?? m.wp_warn_other();

/** "3 days ago" style age of a reading. */
export function ageText(iso: string, locale: string, now = Date.now()): string {
	const hours = Math.round((now - Date.parse(iso)) / 3_600_000);
	const rtf = new Intl.RelativeTimeFormat(locale, { numeric: 'auto' });
	if (Math.abs(hours) < 48) return rtf.format(-hours, 'hour');
	return rtf.format(-Math.round(hours / 24), 'day');
}

/** Freshness in words: never measured, stale, or fresh. Never colour alone. */
export function freshnessText(l: LatestReading, locale: string, now = Date.now()): string {
	if (!l.reading) return m.wp_never_measured();
	const age = ageText(l.reading.measured_at, locale, now);
	return l.stale ? m.wp_stale({ age }) : age;
}

/** Target range in the canonical unit, e.g. "7,5–9 dKH", "≤ 10 mg/L NO3". */
export function targetText(t: Pick<Target, 'min' | 'max'>, unit: string, locale: string): string {
	const f = (v: string) => formatDecimal(v, locale);
	const u = unitLabel(unit);
	const range =
		t.min && t.max
			? `${f(t.min)}–${f(t.max)}`
			: t.min
				? `≥ ${f(t.min)}`
				: t.max
					? `≤ ${f(t.max)}`
					: '';
	return range && u ? `${range} ${u}` : range;
}

// --- Entry form -----------------------------------------------------------------------------

export type ReadingRow = {
	parameter: string;
	value: string | number;
	unit: string;
	method: string;
	testKit: string;
};

/** Preferred input units from the profile (keys of UnitPrefs that are parameters). */
export type UnitPrefs = Partial<Record<'temperature' | 'alkalinity' | 'salinity', string>>;

export function emptyRows(catalog: ParameterType[], prefs: UnitPrefs = {}): ReadingRow[] {
	return catalog.map((p) => {
		const pref = (prefs as Record<string, string | undefined>)[p.code];
		const unit = pref && p.units.some((u) => u.code === pref) ? pref : p.canonical_unit;
		return { parameter: p.code, value: '', unit, method: '', testKit: '' };
	});
}

const blank = (v: string | number) => String(v).trim() === '';

/**
 * Builds the create request from the rows that have a value. `rowIndex[i]` is the form row
 * of `readings[i]`, so API errors such as `readings[1].value` land on the right row.
 */
export function testToCreate(
	rows: ReadingRow[],
	measuredAt: string,
	notes: string
): { body: WaterTestCreate; rowIndex: number[] } {
	const rowIndex: number[] = [];
	const readings: WaterTestCreate['readings'] = [];
	rows.forEach((r, i) => {
		if (blank(r.value)) return;
		rowIndex.push(i);
		readings.push({
			parameter: r.parameter,
			value: decimalInput(String(r.value)),
			unit: r.unit,
			method: (r.method || null) as WaterTestCreate['readings'][number]['method'],
			test_kit: r.testKit.trim() || null
		});
	});
	return {
		body: { measured_at: measuredAt || null, notes: notes.trim() || null, readings },
		rowIndex
	};
}

/** Maps `readings[i].x` field errors to `rows[j].x` keys of the form. */
export function rowErrors(
	fields: Record<string, string>,
	rowIndex: number[]
): Record<string, string> {
	const out: Record<string, string> = {};
	for (const [k, v] of Object.entries(fields)) {
		const match = /^readings\[(\d+)\]\.(\w+)$/.exec(k);
		if (match) out[`${rowIndex[Number(match[1])]}.${match[2]}`] = v;
		else out[k] = v;
	}
	return out;
}

// --- Targets --------------------------------------------------------------------------------

export type TargetRow = {
	parameter: string;
	min: string | number;
	ideal: string | number;
	max: string | number;
	freshnessDays: string | number;
};

export function targetRows(catalog: ParameterType[], current: Target[]): TargetRow[] {
	return catalog.map((p) => {
		const t = current.find((c) => c.parameter === p.code);
		const hours = t?.freshness_hours;
		return {
			parameter: p.code,
			min: t?.min ?? '',
			ideal: t?.ideal ?? '',
			max: t?.max ?? '',
			freshnessDays: hours ? String(Math.round((hours / 24) * 10) / 10) : ''
		};
	});
}

const opt = (v: string | number) => (blank(v) ? null : decimalInput(String(v)));

/** The full target list for PUT (rows left empty have no target); `rowIndex` as above. */
export function targetsToReplace(rows: TargetRow[]): {
	body: { targets: TargetInput[] };
	rowIndex: number[];
} {
	const rowIndex: number[] = [];
	const targets: TargetInput[] = [];
	rows.forEach((r, i) => {
		const days = opt(r.freshnessDays);
		const t: TargetInput = {
			parameter: r.parameter,
			min: opt(r.min),
			ideal: opt(r.ideal),
			max: opt(r.max),
			freshness_hours: days === null ? null : Math.round(Number(days) * 24)
		};
		if (t.min === null && t.ideal === null && t.max === null && t.freshness_hours === null) return;
		rowIndex.push(i);
		targets.push(t);
	});
	return { body: { targets }, rowIndex };
}

export function targetRowErrors(
	fields: Record<string, string>,
	rowIndex: number[]
): Record<string, string> {
	const out: Record<string, string> = {};
	for (const [k, v] of Object.entries(fields)) {
		const match = /^targets\[(\d+)\](?:\.(\w+))?$/.exec(k);
		if (!match) {
			out[k] = v;
			continue;
		}
		const field = match[2] === 'freshness_hours' ? 'freshnessDays' : (match[2] ?? 'min');
		out[`${rowIndex[Number(match[1])]}.${field}`] = v;
	}
	return out;
}

// --- Chart geometry -------------------------------------------------------------------------

/** "Nice" axis ticks (1, 2, 2.5, 5 × 10ⁿ) covering [lo, hi]. */
export function niceTicks(lo: number, hi: number, count = 5): number[] {
	if (!Number.isFinite(lo) || !Number.isFinite(hi)) return [];
	if (lo === hi) {
		const pad = Math.abs(lo) * 0.1 || 1;
		lo -= pad;
		hi += pad;
	}
	const raw = (hi - lo) / Math.max(1, count - 1);
	const pow = 10 ** Math.floor(Math.log10(raw));
	const step = [1, 2, 2.5, 5, 10].map((k) => k * pow).find((s) => s >= raw) ?? 10 * pow;
	const start = Math.floor(lo / step) * step;
	const out: number[] = [];
	for (let v = start; v <= hi + step * 1e-9; v += step) out.push(Number(v.toPrecision(12)));
	if (out.at(-1)! < hi) out.push(Number((out.at(-1)! + step).toPrecision(12)));
	return out;
}

/** Linear scale from domain [d0, d1] to range [r0, r1]. */
export function scale(d0: number, d1: number, r0: number, r1: number): (v: number) => number {
	const k = d1 === d0 ? 0 : (r1 - r0) / (d1 - d0);
	return (v) => r0 + (v - d0) * k;
}

/** Y domain covering the values and the target bounds, so the band is always visible. */
export function valueDomain(
	values: number[],
	targets: Pick<Target, 'min' | 'max'>[]
): [number, number] {
	const all = [...values];
	for (const t of targets) {
		if (t.min) all.push(Number(t.min));
		if (t.max) all.push(Number(t.max));
	}
	if (all.length === 0) return [0, 1];
	return [Math.min(...all), Math.max(...all)];
}
