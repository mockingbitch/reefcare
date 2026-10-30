import { describe, expect, it } from 'vitest';
import {
	defaultRecurrence,
	describeRRule,
	fromRRule,
	monthDayCapped,
	toRRule,
	weekdayOf,
	type RecurrenceForm
} from './recurrence';

const weekly: RecurrenceForm = {
	freq: 'WEEKLY',
	interval: '1',
	days: ['SU'],
	monthDay: '1',
	time: '09:00'
};

describe('toRRule', () => {
	it('matches the API canonical form', () => {
		expect(toRRule(weekly)).toBe('FREQ=WEEKLY;BYDAY=SU;BYHOUR=9;BYMINUTE=0');
		expect(toRRule({ ...weekly, interval: '2', days: ['SU', 'WE'], time: '18:30' })).toBe(
			'FREQ=WEEKLY;INTERVAL=2;BYDAY=WE,SU;BYHOUR=18;BYMINUTE=30'
		);
		expect(toRRule({ ...weekly, freq: 'MONTHLY', monthDay: '28' })).toBe(
			'FREQ=MONTHLY;BYMONTHDAY=28;BYHOUR=9;BYMINUTE=0'
		);
		expect(toRRule({ ...weekly, freq: 'DAILY', interval: '3', days: [] })).toBe(
			'FREQ=DAILY;INTERVAL=3;BYHOUR=9;BYMINUTE=0'
		);
	});

	it('leaves invalid input for the API to report', () => {
		expect(toRRule({ ...weekly, time: 'soon' })).toContain('BYHOUR=soon');
	});
});

describe('fromRRule', () => {
	it('round-trips canonical rules', () => {
		for (const rule of [
			'FREQ=WEEKLY;INTERVAL=2;BYDAY=WE,SU;BYHOUR=18;BYMINUTE=30',
			'FREQ=MONTHLY;BYMONTHDAY=28;BYHOUR=9;BYMINUTE=0',
			'FREQ=DAILY;BYHOUR=7;BYMINUTE=5'
		]) {
			const form = fromRRule(rule);
			expect(form).not.toBeNull();
			expect(toRRule(form!)).toBe(rule);
		}
	});

	it('returns null for rules the form cannot edit', () => {
		expect(fromRRule('FREQ=DAILY;BYHOUR=8,20;BYMINUTE=0')).toBeNull();
		expect(fromRRule('FREQ=MONTHLY;BYMONTHDAY=1,15;BYHOUR=9;BYMINUTE=0')).toBeNull();
		expect(fromRRule('FREQ=YEARLY')).toBeNull();
	});
});

describe('defaults', () => {
	it('uses the start date weekday and caps month days at 28', () => {
		expect(weekdayOf('2026-11-01')).toBe('SU');
		expect(defaultRecurrence('2026-10-29').days).toEqual(['TH']);
		expect(defaultRecurrence('2026-10-31').monthDay).toBe('28');
		expect(monthDayCapped('2026-10-31')).toBe(true);
		expect(monthDayCapped('2026-10-28')).toBe(false);
	});
});

describe('describeRRule', () => {
	it('describes editable rules and falls back to the raw rule', () => {
		expect(describeRRule('FREQ=WEEKLY;BYDAY=SU;BYHOUR=9;BYMINUTE=0')).toMatch(/09:00/);
		expect(describeRRule('FREQ=DAILY;BYHOUR=8,20;BYMINUTE=0')).toBe(
			'FREQ=DAILY;BYHOUR=8,20;BYMINUTE=0'
		);
	});
});
