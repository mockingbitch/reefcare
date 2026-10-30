import { describe, expect, it } from 'vitest';
import { fromLocalInput, percentOfSystem, toLocalInput } from './water-changes';

describe('percentOfSystem', () => {
	it('formats the exact decimal with one decimal place', () => {
		expect(percentOfSystem('0.139', 'en')).toMatch(/^13\.9\s?%$/);
		expect(percentOfSystem('0.139', 'vi')).toMatch(/^13,9\s?%$/);
		expect(percentOfSystem('0.250', 'en')).toMatch(/^25\s?%$/);
		expect(percentOfSystem('1.200', 'en')).toMatch(/^120\s?%$/); // never clamped
	});

	it('shows "< 0.1 %" for a non-zero volume that rounds to zero', () => {
		expect(percentOfSystem('0.000', 'en')).toMatch(/^0\s?%$/);
		expect(percentOfSystem('0.0004', 'en')).toMatch(/^<\s?0[.,]1\s?%$/);
	});
});

describe('local datetime inputs', () => {
	it('round-trips in a fixed-offset zone regardless of the browser zone', () => {
		expect(toLocalInput('2026-10-29T03:00:00Z', 'Asia/Ho_Chi_Minh')).toBe('2026-10-29T10:00');
		expect(fromLocalInput('2026-10-29T10:00', 'Asia/Ho_Chi_Minh')).toBe('2026-10-29T03:00:00Z');
	});

	it('handles zones with daylight saving time', () => {
		// 2026-07-01 12:00 in New York is EDT (UTC-4); 2026-12-01 12:00 is EST (UTC-5).
		expect(fromLocalInput('2026-07-01T12:00', 'America/New_York')).toBe('2026-07-01T16:00:00Z');
		expect(fromLocalInput('2026-12-01T12:00', 'America/New_York')).toBe('2026-12-01T17:00:00Z');
	});

	it('rejects incomplete values', () => {
		expect(fromLocalInput('2026-10-29', 'UTC')).toBe('');
	});
});
