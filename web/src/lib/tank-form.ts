// Form state for creating and editing tanks. The API validates everything; these helpers
// only shape the request and give early UX hints (docs/frontend.md §4).
import type { Tank, TankCreate, TankPatch } from '#lib/api/client.js';

export const tankTypes = [
	'mixed_reef',
	'sps_dominant',
	'lps_dominant',
	'soft_coral',
	'fowlr',
	'fish_only',
	'custom'
] as const;
export const editableStatuses = ['cycling', 'active', 'paused'] as const;

export type TankForm = {
	name: string;
	description: string;
	tank_type: string;
	status: string;
	length: string;
	width: string;
	height: string;
	display_volume_l: string;
	sump_volume_l: string;
	system_volume_l: string;
	setup_date: string;
	salt_mix: string;
	water_source: string;
};

export function emptyTankForm(): TankForm {
	return {
		name: '',
		description: '',
		tank_type: 'mixed_reef',
		status: 'cycling',
		length: '',
		width: '',
		height: '',
		display_volume_l: '',
		sump_volume_l: '',
		system_volume_l: '',
		setup_date: '',
		salt_mix: '',
		water_source: ''
	};
}

export function fromTank(t: Tank): TankForm {
	return {
		name: t.name,
		description: t.description ?? '',
		tank_type: t.tank_type,
		status: t.status,
		length: t.dimensions_cm?.length ?? '',
		width: t.dimensions_cm?.width ?? '',
		height: t.dimensions_cm?.height ?? '',
		display_volume_l: t.display_volume_l,
		sump_volume_l: t.sump_volume_l ?? '',
		system_volume_l: t.system_volume_l,
		setup_date: t.setup_date ?? '',
		salt_mix: t.salt_mix ?? '',
		water_source: t.water_source ?? ''
	};
}

/**
 * Normalizes a typed decimal: trims it and accepts a single comma as the decimal
 * separator ("60,5" → "60.5"), common on Vietnamese keyboards. Anything else is sent as
 * typed so the API reports it precisely.
 */
export function decimalInput(s: string): string {
	const t = s.trim();
	return /^-?\d+,\d+$/.test(t) ? t.replace(',', '.') : t;
}

function opt(s: string): string | null {
	const t = s.trim();
	return t === '' ? null : t;
}

function dimensions(f: TankForm): TankCreate['dimensions_cm'] {
	if ([f.length, f.width, f.height].every((v) => v.trim() === '')) return null;
	return {
		length: decimalInput(f.length),
		width: decimalInput(f.width),
		height: decimalInput(f.height)
	};
}

/** Problems the browser can spot before sending, keyed like API fields. */
export function clientProblems(f: TankForm): Record<string, 'dimensions_all_or_none'> {
	const filled = [f.length, f.width, f.height].filter((v) => v.trim() !== '').length;
	return filled === 0 || filled === 3 ? {} : { dimensions_cm: 'dimensions_all_or_none' };
}

/** A smaller system than display volume is allowed (rock displacement) but worth a warning. */
export function systemBelowDisplay(f: TankForm): boolean {
	const display = Number(decimalInput(f.display_volume_l));
	const system = Number(decimalInput(f.system_volume_l));
	return Number.isFinite(display) && Number.isFinite(system) && system > 0 && system < display;
}

export function toCreate(f: TankForm): TankCreate {
	const sump = opt(f.sump_volume_l);
	return {
		name: f.name.trim(),
		description: opt(f.description),
		tank_type: f.tank_type as TankCreate['tank_type'],
		status: f.status as TankCreate['status'],
		dimensions_cm: dimensions(f),
		display_volume_l: decimalInput(f.display_volume_l),
		sump_volume_l: sump === null ? null : decimalInput(sump),
		system_volume_l: decimalInput(f.system_volume_l),
		setup_date: opt(f.setup_date),
		salt_mix: opt(f.salt_mix),
		water_source: opt(f.water_source)
	};
}

/** A merge patch with only the fields that differ from the original tank. */
export function toPatch(f: TankForm, original: Tank): TankPatch {
	const next = toCreate(f);
	const before = toCreate(fromTank(original));
	const patch: Record<string, unknown> = { version: original.version };
	for (const key of Object.keys(next) as (keyof TankCreate)[]) {
		if (JSON.stringify(next[key]) !== JSON.stringify(before[key])) patch[key] = next[key];
	}
	return patch as TankPatch;
}
