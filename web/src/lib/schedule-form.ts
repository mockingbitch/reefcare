// Form state for water-change schedules. The API validates everything; this only shapes
// requests (create, and merge patches with only the changed fields).
import type {
	WaterChangeSchedule,
	WaterChangeScheduleCreate,
	WaterChangeSchedulePatch
} from '#lib/api/client.js';
import { defaultRecurrence, fromRRule, toRRule, type RecurrenceForm } from '#lib/recurrence.js';
import { decimalInput } from '#lib/tank-form.js';

export type ScheduleForm = {
	starts_on: string;
	ends_on: string;
	/** Null when the stored rule can't be edited with the form; `rrule` is then kept as is. */
	recurrence: RecurrenceForm | null;
	rrule: string;
	timezone: string;
	planned_volume_l: string;
	salt_mix: string;
	notes: string;
};

export function emptyScheduleForm(today: string, timezone: string): ScheduleForm {
	const recurrence = defaultRecurrence(today);
	return {
		starts_on: today,
		ends_on: '',
		recurrence,
		rrule: toRRule(recurrence),
		timezone,
		planned_volume_l: '',
		salt_mix: '',
		notes: ''
	};
}

export function fromSchedule(s: WaterChangeSchedule): ScheduleForm {
	return {
		starts_on: s.starts_on,
		ends_on: s.ends_on ?? '',
		recurrence: fromRRule(s.rrule),
		rrule: s.rrule,
		timezone: s.timezone,
		planned_volume_l: s.planned_volume_l,
		salt_mix: s.salt_mix ?? '',
		notes: s.notes ?? ''
	};
}

const opt = (s: string) => (s.trim() === '' ? null : s.trim());

export function toCreate(f: ScheduleForm): WaterChangeScheduleCreate {
	return {
		rrule: f.recurrence ? toRRule(f.recurrence) : f.rrule,
		timezone: f.timezone,
		starts_on: f.starts_on,
		ends_on: opt(f.ends_on),
		planned_volume_l: decimalInput(f.planned_volume_l),
		salt_mix: opt(f.salt_mix),
		notes: opt(f.notes)
	};
}

/** A merge patch with `version` and only the fields that differ from the stored schedule. */
export function toPatch(f: ScheduleForm, original: WaterChangeSchedule): WaterChangeSchedulePatch {
	const next = toCreate(f);
	const before = toCreate(fromSchedule(original));
	const patch: Record<string, unknown> = { version: original.version };
	for (const key of Object.keys(next) as (keyof WaterChangeScheduleCreate)[]) {
		if (JSON.stringify(next[key]) !== JSON.stringify(before[key])) patch[key] = next[key];
	}
	return patch as WaterChangeSchedulePatch;
}
