// Renders notifications from `kind` + `payload` (api/openapi.yaml Notification). Unknown
// kinds render generically, as the contract asks, so new server kinds never break the list.
import type { Notification } from '#lib/api/client.js';
import { doseText } from '#lib/dosing.js';
import { formatDate, formatDateTime } from '#lib/format.js';
import { m } from '#lib/paraglide/messages.js';

type Payload = Record<string, unknown>;
const str = (p: Payload, k: string) => (typeof p[k] === 'string' ? (p[k] as string) : '');

export function notificationText(n: Notification, locale: string, timeZone: string): string {
	const p = (n.payload ?? {}) as Payload;
	const tank = str(p, 'tank_name');
	const when = (k: string) => (str(p, k) ? formatDateTime(str(p, k), locale, timeZone) : '');
	const day = (k: string) => (str(p, k) ? formatDate(str(p, k), locale) : '');
	switch (n.kind) {
		case 'dosing.missed':
			return m.nt_dosing_missed({
				dose: str(p, 'amount') ? doseText(str(p, 'amount'), str(p, 'unit'), locale) : '',
				product: str(p, 'product_name'),
				tank,
				when: when('scheduled_for')
			});
		case 'water_change.upcoming':
			return m.nt_water_change_upcoming({
				tank,
				when: when('scheduled_for'),
				volume: str(p, 'volume_l')
			});
		case 'water_change.overdue':
			return m.nt_water_change_overdue({ tank, when: when('scheduled_for') });
		case 'maintenance.due':
			return m.nt_maintenance_due({ title: str(p, 'title'), tank });
		case 'maintenance.overdue':
			return m.nt_maintenance_overdue({ title: str(p, 'title'), tank, date: day('due_on') });
		default:
			return tank ? m.nt_generic_tank({ tank }) : m.nt_generic();
	}
}

/** The tank section a notification belongs to, for its link. */
export function notificationSection(
	n: Notification
): 'dosing' | 'water-changes' | 'maintenance' | null {
	if (!n.tank_id) return null;
	switch (n.category) {
		case 'dosing':
			return 'dosing';
		case 'water_change':
			return 'water-changes';
		case 'maintenance':
			return 'maintenance';
		default:
			return null;
	}
}

/** Badge text for the unread count ("99+" when the API caps it). */
export function unreadBadge(count: number, capped: boolean): string {
	if (capped || count > 99) return '99+';
	return String(count);
}
