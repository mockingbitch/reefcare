// Renders tank timeline events (GET /tanks/{id}/timeline) from `event_type` + `summary`.
// New event types may appear; they fall back to their category so the feed never breaks.
import type { TimelineEvent } from '#lib/api/client.js';
import { doseText } from '#lib/dosing.js';
import { categoryLabel } from '#lib/equipment.js';
import { eventLabel } from '#lib/livestock.js';
import { m } from '#lib/paraglide/messages.js';

export type { TimelineEvent };

type Summary = Record<string, unknown>;
const str = (s: Summary, k: string) =>
	typeof s[k] === 'string' || typeof s[k] === 'number' ? String(s[k]) : '';

const categoryLabels: Record<string, () => string> = {
	measurement: m.tl_cat_measurement,
	dosing: m.tl_cat_dosing,
	water_change: m.tl_cat_water_change,
	equipment: m.tl_cat_equipment,
	maintenance: m.tl_cat_maintenance,
	livestock: m.tl_cat_livestock,
	symptom: m.tl_cat_symptom,
	treatment: m.tl_cat_treatment,
	milestone: m.tl_cat_milestone,
	note: m.tl_cat_note,
	assessment: m.tl_cat_assessment
};

export const timelineCategoryLabel = (c: string) => categoryLabels[c]?.() ?? c;

export function timelineText(e: TimelineEvent, locale: string): string {
	const s = (e.summary ?? {}) as Summary;
	const [module, what = ''] = e.event_type.split('.');
	switch (module) {
		case 'tank':
			if (what === 'created') return m.tl_tank_created();
			if (what === 'archived') return m.tl_tank_archived();
			break;
		case 'water_change': {
			const volume = str(s, 'volume_l');
			if (what === 'completed') {
				return volume ? m.tl_water_change_done({ volume }) : m.tl_water_change_done_plain();
			}
			break;
		}
		case 'dosing': {
			const dose = str(s, 'amount') ? doseText(str(s, 'amount'), str(s, 'unit'), locale) : '';
			const labels: Record<string, () => string> = {
				completed: m.dose_status_completed,
				partial: m.dose_status_partial,
				skipped: m.dose_status_skipped,
				failed: m.dose_status_failed,
				missed: m.dose_status_missed
			};
			const status = labels[what]?.() ?? what;
			return dose ? m.tl_dosing_with_dose({ status, dose }) : m.tl_dosing({ status });
		}
		case 'livestock': {
			const name = str(s, 'name');
			const delta = Number(s.quantity_delta ?? 0);
			const qty = delta ? ` (${delta > 0 ? '+' : '−'}${Math.abs(delta)})` : '';
			return m.tl_livestock({ what: eventLabel(what), name, qty });
		}
		case 'equipment': {
			const name = str(s, 'name');
			if (what === 'installed') {
				return m.tl_equipment_installed({ name, category: categoryLabel(str(s, 'category')) });
			}
			if (what === 'removed') return m.tl_equipment_removed({ name });
			if (what === 'configuration_changed') {
				return m.tl_equipment_configured({ version: str(s, 'version') });
			}
			break;
		}
		case 'maintenance':
			if (what === 'performed') return m.tl_maintenance_done({ title: str(s, 'title') });
			break;
	}
	return timelineCategoryLabel(e.category);
}
