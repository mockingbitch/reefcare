// Equipment and maintenance helpers. The API validates everything and never interprets
// device settings; this module shapes requests and compares configuration versions.
import type {
	Equipment,
	EquipmentCreate,
	EquipmentPatch,
	MaintenancePlan,
	MaintenancePlanCreate,
	MaintenancePlanPatch
} from '#lib/api/client.js';
import { m } from '#lib/paraglide/messages.js';

export const categories = [
	'lighting',
	'return_pump',
	'flow_pump',
	'skimmer',
	'heater',
	'chiller',
	'dosing_pump',
	'ato',
	'reactor',
	'uv',
	'controller',
	'filter',
	'sensor',
	'other'
] as const;
export type Category = (typeof categories)[number];

const categoryLabels: Record<Category, () => string> = {
	lighting: m.eq_cat_lighting,
	return_pump: m.eq_cat_return_pump,
	flow_pump: m.eq_cat_flow_pump,
	skimmer: m.eq_cat_skimmer,
	heater: m.eq_cat_heater,
	chiller: m.eq_cat_chiller,
	dosing_pump: m.eq_cat_dosing_pump,
	ato: m.eq_cat_ato,
	reactor: m.eq_cat_reactor,
	uv: m.eq_cat_uv,
	controller: m.eq_cat_controller,
	filter: m.eq_cat_filter,
	sensor: m.eq_cat_sensor,
	other: m.eq_cat_other
};

export const categoryLabel = (c: string) => categoryLabels[c as Category]?.() ?? c;

// --- Settings (free-form key/value, never interpreted) --------------------------------

export type SettingValue = string | number | boolean;
export type Settings = Record<string, SettingValue>;
export type SettingKind = 'text' | 'number' | 'boolean';
export type SettingRow = { key: string; kind: SettingKind; value: string };

export function settingsToRows(s: Settings | null | undefined): SettingRow[] {
	return Object.entries(s ?? {}).map(([key, v]) => ({
		key,
		kind: typeof v === 'number' ? 'number' : typeof v === 'boolean' ? 'boolean' : 'text',
		value: String(v)
	}));
}

/**
 * Builds the settings object. Blank keys are dropped; numbers accept a decimal comma. A
 * number row that doesn't parse is sent as text so the API reports nothing silently lost.
 */
export function rowsToSettings(rows: SettingRow[]): Settings {
	const out: Settings = {};
	for (const r of rows) {
		const key = r.key.trim();
		if (key === '') continue;
		if (r.kind === 'boolean') out[key] = r.value === 'true';
		else if (r.kind === 'number') {
			const t = r.value.trim().replace(/^(-?\d+),(\d+)$/, '$1.$2');
			const n = Number(t);
			out[key] = t !== '' && Number.isFinite(n) ? n : r.value;
		} else out[key] = r.value;
	}
	return out;
}

export type SettingsDiff = { added: string[]; removed: string[]; changed: string[] };

/** Keys added, removed or changed from `before` to `after` (a version's change summary). */
export function diffSettings(before: Settings | null | undefined, after: Settings): SettingsDiff {
	const prev = before ?? {};
	return {
		added: Object.keys(after).filter((k) => !(k in prev)),
		removed: Object.keys(prev).filter((k) => !(k in after)),
		changed: Object.keys(after).filter((k) => k in prev && prev[k] !== after[k])
	};
}

export function formatSetting(v: SettingValue, locale: string): string {
	if (typeof v === 'boolean') return v ? m.yes() : m.no();
	if (typeof v === 'number') return new Intl.NumberFormat(locale).format(v);
	return v;
}

// --- Equipment form ----------------------------------------------------------------------

export type EquipmentForm = {
	category: Category;
	name: string;
	manufacturer: string;
	model: string;
	serial_number: string;
	purchased_on: string;
	warranty_until: string;
	/** `datetime-local` in the user's zone; only used on create (fixed afterwards). */
	installed_at: string;
	notes: string;
};

export function emptyEquipmentForm(installedAt: string): EquipmentForm {
	return {
		category: 'lighting',
		name: '',
		manufacturer: '',
		model: '',
		serial_number: '',
		purchased_on: '',
		warranty_until: '',
		installed_at: installedAt,
		notes: ''
	};
}

export function fromEquipment(e: Equipment): EquipmentForm {
	return {
		category: e.category,
		name: e.name,
		manufacturer: e.manufacturer ?? '',
		model: e.model ?? '',
		serial_number: e.serial_number ?? '',
		purchased_on: e.purchased_on ?? '',
		warranty_until: e.warranty_until ?? '',
		installed_at: '',
		notes: e.notes ?? ''
	};
}

const opt = (s: string) => (s.trim() === '' ? null : s.trim());

function common(f: EquipmentForm) {
	return {
		category: f.category,
		name: f.name.trim(),
		manufacturer: opt(f.manufacturer),
		model: opt(f.model),
		serial_number: opt(f.serial_number),
		purchased_on: opt(f.purchased_on),
		warranty_until: opt(f.warranty_until),
		notes: opt(f.notes)
	};
}

/** `installedAt` is the RFC 3339 instant converted from the form's local value. */
export function equipmentToCreate(f: EquipmentForm, installedAt: string): EquipmentCreate {
	return { ...common(f), ...(installedAt ? { installed_at: installedAt } : {}) };
}

export function equipmentToPatch(f: EquipmentForm, original: Equipment): EquipmentPatch {
	const next = common(f);
	const before = common(fromEquipment(original));
	const patch: Record<string, unknown> = { version: original.version };
	for (const key of Object.keys(next) as (keyof typeof next)[]) {
		if (next[key] !== before[key]) patch[key] = next[key];
	}
	return patch as EquipmentPatch;
}

/** True when a warranty end date has passed (calendar dates, no zone shift). */
export function warrantyExpired(until: string | null, today: string): boolean {
	return until !== null && until < today;
}

// --- Maintenance plan form ---------------------------------------------------------------

export type MaintenancePlanForm = {
	title: string;
	/** `<input type="number">` binds a number; either is accepted. */
	interval_days: string | number;
	timezone: string;
	starts_on: string;
	equipment_id: string;
	notes: string;
};

export function emptyMaintenanceForm(today: string, timezone: string): MaintenancePlanForm {
	return {
		title: '',
		interval_days: '30',
		timezone,
		starts_on: today,
		equipment_id: '',
		notes: ''
	};
}

export function fromMaintenancePlan(p: MaintenancePlan): MaintenancePlanForm {
	return {
		title: p.title,
		interval_days: String(p.interval_days),
		timezone: p.timezone,
		starts_on: p.starts_on,
		equipment_id: p.equipment_id ?? '',
		notes: p.notes ?? ''
	};
}

export function maintenanceToCreate(f: MaintenancePlanForm): MaintenancePlanCreate {
	const n = Number(String(f.interval_days ?? '').trim());
	return {
		title: f.title.trim(),
		// A non-integer is sent unchanged in type terms (NaN → 0) so the API reports the field.
		interval_days: Number.isInteger(n) ? n : 0,
		timezone: f.timezone,
		starts_on: f.starts_on,
		equipment_id: f.equipment_id || null,
		notes: opt(f.notes)
	};
}

export function maintenanceToPatch(
	f: MaintenancePlanForm,
	original: MaintenancePlan
): MaintenancePlanPatch {
	const next = maintenanceToCreate(f);
	const before = maintenanceToCreate(fromMaintenancePlan(original));
	const patch: Record<string, unknown> = { version: original.version };
	for (const key of Object.keys(next) as (keyof MaintenancePlanCreate)[]) {
		if (next[key] !== before[key]) patch[key] = next[key];
	}
	return patch as MaintenancePlanPatch;
}
