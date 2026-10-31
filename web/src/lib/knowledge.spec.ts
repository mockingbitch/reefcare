import { describe, expect, it } from 'vitest';
import { safeHref, structuredToText, tags, textToStructured } from './knowledge';

describe('knowledge', () => {
	it('round-trips structured lists through text', () => {
		const s = {
			verifications: [{ key: 'retest', text: 'Re-test with another kit' }],
			recommendations: [
				{ key: 'steady', text: 'Keep dosing steady', claim_class: 'expert' as const }
			],
			contraindications: [],
			safety_warnings: [{ key: 'slow', text: 'Raise KH slowly | at most 1 dKH/day' }]
		};
		const back = textToStructured(structuredToText(s));
		expect(back.errors).toEqual([]);
		expect(back.structured).toEqual(s);
	});

	it('reports malformed lines instead of dropping them silently', () => {
		const r = textToStructured({
			verifications: 'no separator\n\nok | fine',
			recommendations: 'k | rumour | text\nk2 | expert',
			contraindications: '',
			safety_warnings: ''
		});
		expect(r.errors).toEqual(['verifications:1', 'recommendations:1', 'recommendations:2']);
		expect(r.structured.verifications).toEqual([{ key: 'ok', text: 'fine' }]);
	});

	it('splits tags and only links https', () => {
		expect(tags(' alkalinity, calcium  ph ')).toEqual(['alkalinity', 'calcium', 'ph']);
		expect(safeHref('https://a.example')).toBe('https://a.example');
		expect(safeHref('javascript:alert(1)')).toBeNull();
		expect(safeHref(null)).toBeNull();
	});
});
