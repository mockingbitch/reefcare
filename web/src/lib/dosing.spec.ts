import { describe, expect, it } from 'vitest';
import type { DoseEvent, DosingPlan } from '#lib/api/client.js';
import {
	adHocWarning,
	doseText,
	dueSoon,
	nearbyWindowHours,
	emptyPlanForm,
	fromPlan,
	nearbyDoses,
	planToCreate,
	planToPatch,
	unitsForForm
} from './dosing';

const product = { id: 'p1', name: 'Alk part A', brand: null, form: 'liquid' as const };

const plan: DosingPlan = {
	id: 'plan1',
	tank_id: 't1',
	product,
	amount: '10',
	amount_unit: 'ml',
	rrule: 'FREQ=DAILY;BYHOUR=8;BYMINUTE=0',
	timezone: 'Asia/Ho_Chi_Minh',
	starts_on: '2026-10-01',
	ends_on: null,
	delivery_method: 'manual',
	status: 'active',
	notes: null,
	next_occurrences: [],
	created_at: '2026-10-01T00:00:00Z',
	updated_at: '2026-10-01T00:00:00Z',
	version: 2
};

function event(id: string, at: string, status: DoseEvent['status'] = 'completed'): DoseEvent {
	return {
		id,
		tank_id: 't1',
		plan_id: 'plan1',
		product,
		scheduled_for: at,
		planned_amount: '10',
		actual_amount: null,
		actual_source: null,
		deviation: null,
		amount_unit: 'ml',
		status,
		executed_at: status === 'completed' ? at : null,
		reason: null,
		reason_code: null,
		notes: null,
		reported_by: null,
		missed_at: null,
		resolved_at: null,
		created_at: at,
		updated_at: at,
		version: 1
	};
}

describe('units', () => {
	it('follow the product form like the API', () => {
		expect(unitsForForm('liquid')).toEqual(['ml', 'l', 'drops']);
		expect(unitsForForm('tablet')).toEqual(['tablets']);
		expect(unitsForForm('powder')).toContain('g');
		expect(unitsForForm('other')).toHaveLength(6);
	});

	it('format with the locale separator', () => {
		expect(doseText('12.5', 'ml', 'vi')).toMatch(/^12,5 ml$/);
		expect(doseText('12.5', 'ml', 'en')).toMatch(/^12\.5 ml$/);
	});
});

const daily = {
	...plan,
	next_occurrences: ['2026-10-31T01:00:00Z', '2026-11-01T01:00:00Z', '2026-11-02T01:00:00Z']
};
const pump48 = {
	...plan,
	id: 'pump',
	amount: '0.5',
	delivery_method: 'dosing_pump' as const,
	next_occurrences: ['2026-10-31T01:00:00Z', '2026-10-31T01:30:00Z', '2026-10-31T02:00:00Z']
};

describe('adHocWarning', () => {
	it('flags much more than the plan, or a unit no plan uses', () => {
		expect(adHocWarning('p1', '25', 'ml', [plan])).toBeNull();
		expect(adHocWarning('p1', '30,5', 'ml', [plan])).toBe('over_plan');
		expect(adHocWarning('p1', '5', 'drops', [plan])).toBe('unusual_unit');
		expect(adHocWarning('other', '500', 'ml', [plan])).toBeNull();
		expect(adHocWarning('p1', '500', 'ml', [{ ...plan, status: 'paused' }])).toBeNull();
	});

	it('compares with the largest plan, and with the daily total of pump plans', () => {
		const small = { ...plan, id: 'small', amount: '2' };
		expect(adHocWarning('p1', '10', 'ml', [small, plan])).toBeNull(); // 10 ≤ 3 × 10
		// 0.5 ml × 48 a day = 24 ml: a 20 ml manual dose is not unusual.
		expect(adHocWarning('p1', '20', 'ml', [pump48])).toBeNull();
		expect(adHocWarning('p1', '80', 'ml', [pump48])).toBe('over_plan');
	});
});

describe('nearbyDoses', () => {
	it('finds other doses of the product within the window, ignoring never-given ones', () => {
		const events = [
			event('a', '2026-10-30T01:00:00Z'),
			event('b', '2026-10-30T12:00:00Z'),
			event('c', '2026-10-30T02:00:00Z', 'skipped'),
			event('d', '2026-10-30T02:30:00Z', 'missed')
		];
		expect(nearbyDoses('2026-10-30T03:00:00Z', 'p1', events, 'x').map((e) => e.id)).toEqual(['a']);
		expect(nearbyDoses('2026-10-30T03:00:00Z', 'p1', events, 'a')).toEqual([]);
	});

	it('scales the window with the plan spacing', () => {
		expect(nearbyWindowHours(daily)).toBe(12);
		expect(nearbyWindowHours(undefined)).toBe(6);
		expect(nearbyWindowHours(pump48)).toBeNull(); // micro-dosing: no warning
	});
});

describe('dueSoon', () => {
	it('allows one-tap confirmation only for doses due within 6 hours', () => {
		const now = Date.parse('2026-10-30T00:00:00Z');
		expect(dueSoon(event('a', '2026-10-30T05:00:00Z', 'scheduled'), now)).toBe(true);
		expect(dueSoon(event('b', '2026-10-31T00:00:00Z', 'scheduled'), now)).toBe(false);
	});
});

describe('plan form', () => {
	it('creates with a decimal comma and patches only changes without the product', () => {
		const f = emptyPlanForm('2026-11-01', 'Asia/Ho_Chi_Minh');
		f.product_id = 'p1';
		f.amount = '7,5';
		expect(planToCreate(f)).toMatchObject({
			product_id: 'p1',
			amount: '7.5',
			rrule: 'FREQ=DAILY;BYHOUR=9;BYMINUTE=0',
			delivery_method: 'manual',
			ends_on: null
		});

		const edit = fromPlan(plan);
		expect(planToPatch(edit, plan)).toEqual({ version: 2 });
		edit.amount = '12';
		edit.product_id = 'changed';
		expect(planToPatch(edit, plan)).toEqual({ version: 2, amount: '12' });
	});
});
