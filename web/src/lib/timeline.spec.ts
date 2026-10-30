import { describe, expect, it } from 'vitest';
import { timelineText, type TimelineEvent } from './timeline';

function ev(
	event_type: string,
	category: TimelineEvent['category'],
	summary: object
): TimelineEvent {
	return {
		id: 'e1',
		occurred_at: '2026-10-30T01:00:00Z',
		category,
		event_type,
		source: { type: 'x', id: 'y' },
		actor: null,
		summary: summary as TimelineEvent['summary']
	};
}

describe('timelineText', () => {
	it('renders known event types from their summary', () => {
		expect(
			timelineText(ev('dosing.completed', 'dosing', { amount: '12.5', unit: 'ml' }), 'vi')
		).toContain('12,5 ml');
		expect(
			timelineText(ev('water_change.completed', 'water_change', { volume_l: '10' }), 'en')
		).toContain('10');
		expect(
			timelineText(
				ev('livestock.died', 'livestock', { name: 'Clownfish', quantity_delta: -1 }),
				'en'
			)
		).toContain('Clownfish (−1)');
		expect(
			timelineText(ev('maintenance.performed', 'maintenance', { title: 'Clean lens' }), 'en')
		).toContain('Clean lens');
	});

	it('falls back to the category for unknown types', () => {
		expect(timelineText(ev('assessment.created', 'assessment', {}), 'en')).not.toBe('');
		expect(timelineText(ev('water_change.something_new', 'water_change', {}), 'en')).not.toBe('');
	});
});
