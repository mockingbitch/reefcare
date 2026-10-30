// Dosing helpers. The API owns every rule (units per product form, over-dose confirmation,
// late-dose timing); this module shapes requests and adds the non-blocking UI hints from
// the domain review (docs/open-items.md DS5). ReefCare never converts units or infers
// concentrations, and never controls a pump.
import type {
	DoseEvent,
	DoseUnit,
	DosingForm,
	DosingPlan,
	DosingPlanCreate,
	DosingPlanPatch
} from '#lib/api/client.js';
import { m } from '#lib/paraglide/messages.js';
import { defaultRecurrence, fromRRule, toRRule, type RecurrenceForm } from '#lib/recurrence.js';
import { decimalInput } from '#lib/tank-form.js';

export const allUnits: DoseUnit[] = ['ml', 'l', 'drops', 'g', 'mg', 'tablets'];

/** Units the API accepts for a product form (api/openapi.yaml DoseUnit). */
export function unitsForForm(form: DosingForm | undefined): DoseUnit[] {
	switch (form) {
		case 'liquid':
			return ['ml', 'l', 'drops'];
		case 'powder':
			return ['g', 'mg', 'ml', 'l']; // ml/l: dissolved ("prepared solution")
		case 'tablet':
			return ['tablets'];
		default:
			return allUnits;
	}
}

const unitLabels: Record<DoseUnit, () => string> = {
	ml: m.unit_ml,
	l: m.unit_l,
	drops: m.unit_drops,
	g: m.unit_g,
	mg: m.unit_mg,
	tablets: m.unit_tablets
};

export const unitLabel = (u: string) => unitLabels[u as DoseUnit]?.() ?? u;

/** "12,5 ml" with the locale's decimal separator. */
export function doseText(amount: string, unit: string, locale: string): string {
	const sep = locale.startsWith('vi') ? ',' : '.';
	return m.dose_amount({ amount: amount.replace('.', sep), unit: unitLabel(unit) });
}

const hourMs = 3_600_000;
const dayMs = 24 * hourMs;

/** Time between a plan's next two occurrences, or null when fewer than two are known. */
export function planSpacingMs(p: DosingPlan): number | null {
	const [a, b] = p.next_occurrences;
	if (!a || !b) return null;
	const gap = Date.parse(b) - Date.parse(a);
	return gap > 0 ? gap : null;
}

/**
 * What one dose of the plan is compared with: the per-occurrence amount, or for a pump plan
 * dosing several times a day the daily total (micro-dosing amounts are tiny per occurrence).
 */
function referenceAmount(p: DosingPlan): number {
	const amount = Number(p.amount);
	const spacing = planSpacingMs(p);
	if (p.delivery_method === 'dosing_pump' && spacing && spacing < dayMs) {
		return amount * Math.max(1, Math.round(dayMs / spacing));
	}
	return amount;
}

/**
 * Heuristic warning before recording an ad-hoc dose: more than 3× the largest active plan
 * for the same product and unit (pump plans: their daily total), or a unit none of those
 * plans uses (ml vs L is a 1000× trap). Never blocking. Products without an active plan get
 * no check.
 */
export function adHocWarning(
	productId: string,
	amount: string,
	unit: string,
	plans: DosingPlan[]
): 'over_plan' | 'unusual_unit' | null {
	const same = plans.filter((p) => p.status === 'active' && p.product.id === productId);
	if (same.length === 0) return null;
	const sameUnit = same.filter((p) => p.amount_unit === unit);
	if (sameUnit.length === 0) return 'unusual_unit';
	const value = Number(decimalInput(amount));
	if (!Number.isFinite(value)) return null;
	const reference = Math.max(...sameUnit.map(referenceAmount));
	return value > 3 * reference ? 'over_plan' : null;
}

/**
 * Hours around a late dose in which another dose of the same product is worth a warning:
 * half the plan's spacing (12 h for daily, 6 h for twice daily), at least 6 h. Null for
 * plans dosing more often than every 4 h (micro-dosing pumps), where a warning would fire
 * every time and teach people to ignore it.
 */
export function nearbyWindowHours(plan: DosingPlan | undefined): number | null {
	const spacing = plan ? planSpacingMs(plan) : null;
	if (spacing === null) return 6;
	if (spacing < 4 * hourMs) return null;
	return Math.max(6, spacing / 2 / hourMs);
}

/**
 * Other doses of the same product within `hours` of an instant (a late dose may double up).
 * Doses that were never given (missed, skipped, failed) don't count.
 */
export function nearbyDoses(
	at: string,
	productId: string,
	events: DoseEvent[],
	excludeId: string,
	hours = 6
): DoseEvent[] {
	const t = Date.parse(at);
	if (Number.isNaN(t)) return [];
	const window = hours * hourMs;
	return events.filter((e) => {
		if (e.id === excludeId || e.product.id !== productId) return false;
		if (e.status === 'skipped' || e.status === 'failed' || e.status === 'missed') return false;
		const when = e.executed_at ?? e.scheduled_for;
		return when !== null && Math.abs(Date.parse(when) - t) <= window;
	});
}

/** A scheduled dose more than this far ahead needs an explicit "record it now" step. */
export const quickConfirmAheadMs = 6 * hourMs;

/** True when a scheduled dose is due soon enough for the one-tap "Dosed …?" button. */
export function dueSoon(e: DoseEvent, now = Date.now()): boolean {
	return !e.scheduled_for || Date.parse(e.scheduled_for) - now <= quickConfirmAheadMs;
}

export const isLate = (e: DoseEvent) => e.status === 'missed' || e.status === 'unconfirmed';

// --- Plan form ---------------------------------------------------------------------------

export type PlanForm = {
	product_id: string;
	amount: string;
	amount_unit: DoseUnit;
	delivery_method: 'manual' | 'dosing_pump';
	starts_on: string;
	ends_on: string;
	/** Null when the stored rule can't be edited with the form; `rrule` is kept as is. */
	recurrence: RecurrenceForm | null;
	rrule: string;
	timezone: string;
	notes: string;
};

export function emptyPlanForm(today: string, timezone: string): PlanForm {
	const recurrence = { ...defaultRecurrence(today), freq: 'DAILY' as const, days: [] };
	return {
		product_id: '',
		amount: '',
		amount_unit: 'ml',
		delivery_method: 'manual',
		starts_on: today,
		ends_on: '',
		recurrence,
		rrule: toRRule(recurrence),
		timezone,
		notes: ''
	};
}

export function fromPlan(p: DosingPlan): PlanForm {
	return {
		product_id: p.product.id,
		amount: p.amount,
		amount_unit: p.amount_unit,
		delivery_method: p.delivery_method,
		starts_on: p.starts_on,
		ends_on: p.ends_on ?? '',
		recurrence: fromRRule(p.rrule),
		rrule: p.rrule,
		timezone: p.timezone,
		notes: p.notes ?? ''
	};
}

const opt = (s: string) => (s.trim() === '' ? null : s.trim());

export function planToCreate(f: PlanForm): DosingPlanCreate {
	return {
		product_id: f.product_id,
		amount: decimalInput(f.amount),
		amount_unit: f.amount_unit,
		rrule: f.recurrence ? toRRule(f.recurrence) : f.rrule,
		timezone: f.timezone,
		starts_on: f.starts_on,
		ends_on: opt(f.ends_on),
		delivery_method: f.delivery_method,
		notes: opt(f.notes)
	};
}

/** Merge patch with `version` and only changed fields; the product can't change. */
export function planToPatch(f: PlanForm, original: DosingPlan): DosingPlanPatch {
	const next = planToCreate(f);
	const before = planToCreate(fromPlan(original));
	const patch: Record<string, unknown> = { version: original.version };
	for (const key of Object.keys(next) as (keyof DosingPlanCreate)[]) {
		if (key === 'product_id') continue;
		if (JSON.stringify(next[key]) !== JSON.stringify(before[key])) patch[key] = next[key];
	}
	return patch as DosingPlanPatch;
}
