import { describe, expect, it } from 'vitest';
import type { Notification } from '#lib/api/client.js';
import { notificationSection, notificationText, unreadBadge } from './notifications';

function n(kind: string, category: Notification['category'], payload: object): Notification {
	return {
		id: 'n1',
		tank_id: 't1',
		category,
		kind,
		payload: payload as Notification['payload'],
		occurred_at: '2026-10-30T01:00:00Z',
		read_at: null,
		created_at: '2026-10-30T01:00:00Z'
	};
}

describe('notifications', () => {
	it('renders known kinds from their payload', () => {
		const missed = n('dosing.missed', 'dosing', {
			tank_name: 'Nano',
			product_name: 'Alk part A',
			amount: '12.5',
			unit: 'ml',
			scheduled_for: '2026-10-30T01:00:00Z'
		});
		const text = notificationText(missed, 'vi', 'Asia/Ho_Chi_Minh');
		expect(text).toContain('Alk part A');
		expect(text).toContain('12,5 ml');
		expect(text).toContain('Nano');
		expect(notificationSection(missed)).toBe('dosing');

		const overdue = n('maintenance.overdue', 'maintenance', {
			tank_name: 'Nano',
			title: 'Clean lens',
			due_on: '2026-10-28'
		});
		expect(notificationText(overdue, 'en', 'UTC')).toContain('Clean lens');
		expect(notificationSection(overdue)).toBe('maintenance');
	});

	it('renders unknown kinds generically without failing', () => {
		const future = n('assessment.ready', 'dosing', { tank_name: 'Nano' });
		expect(notificationText(future, 'en', 'UTC')).toContain('Nano');
		expect(notificationText({ ...future, payload: {} }, 'en', 'UTC')).not.toBe('');
	});

	it('caps the badge like the API', () => {
		expect(unreadBadge(3, false)).toBe('3');
		expect(unreadBadge(100, true)).toBe('99+');
	});
});
