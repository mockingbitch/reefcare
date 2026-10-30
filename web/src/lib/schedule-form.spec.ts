import { describe, expect, it } from 'vitest';
import type { WaterChangeSchedule } from '#lib/api/client.js';
import { emptyScheduleForm, fromSchedule, toCreate, toPatch } from './schedule-form';

const stored: WaterChangeSchedule = {
	id: '0192f0c4-0000-7000-8000-000000000010',
	tank_id: '0192f0c4-0000-7000-8000-000000000001',
	rrule: 'FREQ=WEEKLY;BYDAY=SU;BYHOUR=9;BYMINUTE=0',
	timezone: 'Asia/Ho_Chi_Minh',
	starts_on: '2026-10-01',
	ends_on: null,
	planned_volume_l: '10',
	salt_mix: null,
	notes: null,
	status: 'active',
	next_occurrences: [],
	estimate: null,
	created_at: '2026-10-01T00:00:00Z',
	updated_at: '2026-10-01T00:00:00Z',
	version: 3
};

describe('schedule form', () => {
	it('builds a create request with a decimal comma and empty optionals as null', () => {
		const f = emptyScheduleForm('2026-11-01', 'Asia/Ho_Chi_Minh');
		f.planned_volume_l = ' 12,5 ';
		expect(toCreate(f)).toEqual({
			rrule: 'FREQ=WEEKLY;BYDAY=SU;BYHOUR=9;BYMINUTE=0',
			timezone: 'Asia/Ho_Chi_Minh',
			starts_on: '2026-11-01',
			ends_on: null,
			planned_volume_l: '12.5',
			salt_mix: null,
			notes: null
		});
	});

	it('patches only changed fields, with the version', () => {
		const f = fromSchedule(stored);
		expect(toPatch(f, stored)).toEqual({ version: 3 });
		f.recurrence!.time = '18:30';
		f.salt_mix = 'Red Sea Coral Pro';
		expect(toPatch(f, stored)).toEqual({
			version: 3,
			rrule: 'FREQ=WEEKLY;BYDAY=SU;BYHOUR=18;BYMINUTE=30',
			salt_mix: 'Red Sea Coral Pro'
		});
	});

	it('keeps rules the form cannot edit untouched', () => {
		const multi = { ...stored, rrule: 'FREQ=DAILY;BYHOUR=8,20;BYMINUTE=0' };
		const f = fromSchedule(multi);
		expect(f.recurrence).toBeNull();
		f.notes = 'twice daily top-off check';
		expect(toPatch(f, multi)).toEqual({ version: 3, notes: 'twice daily top-off check' });
	});
});
