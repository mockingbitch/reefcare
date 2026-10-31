import { describe, expect, it } from 'vitest';
import type { Finding } from '#lib/api/client.js';
import {
	assessmentText,
	evidenceCount,
	messageName,
	orderedWarnings,
	severityIcon
} from './assessments';

describe('assessments', () => {
	it('renders keys with translated parameters and locale numbers', () => {
		const text = assessmentText(
			{
				key: 'pd.observation.below',
				params: { parameter: 'alkalinity', value: '7.4', unit: 'dKH', bound: '7.8' }
			},
			'vi'
		);
		expect(text).toContain('7,4');
		expect(text).toContain('7,8');
		expect(text).toContain('Kiềm');
		expect(messageName('nh.summary.urgent')).toBe('ia_nh_summary_urgent');
	});

	it('renders unknown keys generically and keeps editor text as given', () => {
		expect(assessmentText({ key: 'future.rule.text' }, 'vi')).not.toContain('future');
		expect(
			assessmentText(
				{ key: 'knowledge.safety_warning', params: { text: '<b>x</b>', slug: 's' } },
				'vi'
			)
		).toBe('<b>x</b>');
	});

	it('orders urgent warnings first and counts distinct evidence', () => {
		const f = {
			safety_warnings: [
				{ text: { key: 'a' }, urgent: false },
				{ text: { key: 'b' }, urgent: true }
			],
			observations: [
				{
					text: { key: 'x' },
					evidence: [
						{ kind: 'reading', id: '1' },
						{ kind: 'reading', id: '2' }
					]
				},
				{ text: { key: 'y' }, evidence: [{ kind: 'reading', id: '1' }] }
			]
		} as unknown as Finding;
		expect(orderedWarnings(f)[0].urgent).toBe(true);
		expect(evidenceCount(f)).toBe(2);
		expect(severityIcon('urgent')).toBe('⛔');
	});
});
