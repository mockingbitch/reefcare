// Form model for the recurrence subset the API accepts (api/openapi.yaml RRule). The API
// validates the rule and computes occurrences (GET /recurrence/preview); this module only
// converts between the form and the canonical RRULE string.
import { m } from '#lib/paraglide/messages.js';

export const weekdays = ['MO', 'TU', 'WE', 'TH', 'FR', 'SA', 'SU'] as const;
export type Weekday = (typeof weekdays)[number];
export type Freq = 'DAILY' | 'WEEKLY' | 'MONTHLY';

/** Highest BYMONTHDAY the API accepts: every month has it. */
export const maxMonthDay = 28;

export type RecurrenceForm = {
	freq: Freq;
	interval: string;
	days: Weekday[];
	monthDay: string;
	/** Wall-clock time "HH:MM" in the schedule's time zone. */
	time: string;
};

const jsWeekday: Weekday[] = ['SU', 'MO', 'TU', 'WE', 'TH', 'FR', 'SA'];

/** Weekday of a calendar date (YYYY-MM-DD), independent of the browser's zone. */
export function weekdayOf(date: string): Weekday {
	const [y, mo, d] = date.split('-').map(Number);
	return jsWeekday[new Date(Date.UTC(y, mo - 1, d)).getUTCDay()];
}

/** A sensible default: weekly on the start date's weekday (or its day of month), 09:00. */
export function defaultRecurrence(startsOn: string): RecurrenceForm {
	const day = Number(startsOn.split('-')[2]) || 1;
	return {
		freq: 'WEEKLY',
		interval: '1',
		days: startsOn ? [weekdayOf(startsOn)] : ['SU'],
		monthDay: String(Math.min(day, maxMonthDay)),
		time: '09:00'
	};
}

/**
 * True when a monthly schedule's start falls on the 29th–31st: the API only accepts days
 * 1–28, so the form proposes the 28th and says so (open item W6).
 */
export function monthDayCapped(startsOn: string): boolean {
	return Number(startsOn.split('-')[2]) > maxMonthDay;
}

/** Builds the RRULE in the API's canonical part order. Invalid input is passed through so the API reports it. */
export function toRRule(f: RecurrenceForm): string {
	const parts = [`FREQ=${f.freq}`];
	const interval = f.interval.trim();
	if (interval !== '' && interval !== '1') parts.push(`INTERVAL=${interval}`);
	if (f.freq === 'WEEKLY' && f.days.length > 0) {
		parts.push(`BYDAY=${weekdays.filter((d) => f.days.includes(d)).join(',')}`);
	}
	if (f.freq === 'MONTHLY') parts.push(`BYMONTHDAY=${f.monthDay.trim()}`);
	const [h, min] = splitTime(f.time);
	parts.push(`BYHOUR=${h}`, `BYMINUTE=${min}`);
	return parts.join(';');
}

function splitTime(t: string): [string, string] {
	const match = /^(\d{1,2}):(\d{2})$/.exec(t.trim());
	if (!match) return [t.trim(), '0'];
	return [String(Number(match[1])), String(Number(match[2]))];
}

/**
 * Parses a canonical RRULE back into the form. Returns null for rules the form cannot
 * represent (several times a day, several month days), which are then shown read-only.
 */
export function fromRRule(rrule: string): RecurrenceForm | null {
	const parts = new Map(
		rrule.split(';').map((p) => {
			const [k, v = ''] = p.split('=');
			return [k.toUpperCase(), v] as const;
		})
	);
	const freq = parts.get('FREQ') as Freq | undefined;
	if (freq !== 'DAILY' && freq !== 'WEEKLY' && freq !== 'MONTHLY') return null;
	const hours = (parts.get('BYHOUR') ?? '9').split(',');
	const monthDays = (parts.get('BYMONTHDAY') ?? '').split(',').filter(Boolean);
	if (hours.length !== 1 || monthDays.length > 1) return null;
	const days = (parts.get('BYDAY') ?? '').split(',').filter(Boolean) as Weekday[];
	if (days.some((d) => !weekdays.includes(d))) return null;
	const minute = parts.get('BYMINUTE') ?? '0';
	return {
		freq,
		interval: parts.get('INTERVAL') ?? '1',
		days,
		monthDay: monthDays[0] ?? '1',
		time: `${hours[0].padStart(2, '0')}:${minute.padStart(2, '0')}`
	};
}

const dayNames: Record<Weekday, () => string> = {
	MO: m.weekday_mo,
	TU: m.weekday_tu,
	WE: m.weekday_we,
	TH: m.weekday_th,
	FR: m.weekday_fr,
	SA: m.weekday_sa,
	SU: m.weekday_su
};

export const weekdayLabel = (d: Weekday) => dayNames[d]();

/** Human description, e.g. "Every 2 weeks on Sunday at 09:00". Falls back to the raw rule. */
export function describeRRule(rrule: string): string {
	const f = fromRRule(rrule);
	if (!f) return rrule;
	const n = Number(f.interval) || 1;
	const time = f.time;
	switch (f.freq) {
		case 'DAILY':
			return n === 1 ? m.rec_daily({ time }) : m.rec_every_n_days({ n: String(n), time });
		case 'WEEKLY': {
			const days = f.days.map(weekdayLabel).join(', ');
			return n === 1
				? m.rec_weekly({ days, time })
				: m.rec_every_n_weeks({ n: String(n), days, time });
		}
		case 'MONTHLY':
			return n === 1
				? m.rec_monthly({ day: f.monthDay, time })
				: m.rec_every_n_months({ n: String(n), day: f.monthDay, time });
	}
}
