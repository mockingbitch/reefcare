// Livestock helpers. Quantities change only through events (and reconciliation); the API
// keeps the balance valid in event order. ReefCare records observations; it never diagnoses.
import type {
	LivestockEntry,
	LivestockEntryCreate,
	LivestockEntryPatch,
	Species
} from '#lib/api/client.js';
import { m } from '#lib/paraglide/messages.js';

export const livestockCategories = ['fish', 'coral', 'invertebrate', 'algae', 'other'] as const;
export type LivestockCategory = (typeof livestockCategories)[number];
export const coralTypes = ['sps', 'lps', 'soft', 'non_photosynthetic'] as const;
export const livestockForms = ['individual', 'group', 'frag', 'colony'] as const;

const categoryLabels: Record<LivestockCategory, () => string> = {
	fish: m.ls_cat_fish,
	coral: m.ls_cat_coral,
	invertebrate: m.ls_cat_invertebrate,
	algae: m.ls_cat_algae,
	other: m.ls_cat_other
};
const coralTypeLabels: Record<string, () => string> = {
	sps: m.ls_coral_sps,
	lps: m.ls_coral_lps,
	soft: m.ls_coral_soft,
	non_photosynthetic: m.ls_coral_nps
};
const formLabels: Record<string, () => string> = {
	individual: m.ls_form_individual,
	group: m.ls_form_group,
	frag: m.ls_form_frag,
	colony: m.ls_form_colony
};

export const livestockCategoryLabel = (c: string) =>
	categoryLabels[c as LivestockCategory]?.() ?? c;
export const coralTypeLabel = (c: string) => coralTypeLabels[c]?.() ?? c;
export const livestockFormLabel = (f: string) => formLabels[f]?.() ?? f;

// --- Events ---------------------------------------------------------------------------------

export const arrivalTypes = ['added', 'transferred_in', 'propagated', 'found'] as const;
export const lossTypes = ['died', 'missing', 'transferred_out', 'sold'] as const;
export const observationTypes = ['health_observation', 'treatment'] as const;
export type RecordableEvent =
	(typeof arrivalTypes)[number] | (typeof lossTypes)[number] | (typeof observationTypes)[number];

const eventLabels: Record<string, () => string> = {
	added: m.ls_ev_added,
	transferred_in: m.ls_ev_transferred_in,
	propagated: m.ls_ev_propagated,
	found: m.ls_ev_found,
	died: m.ls_ev_died,
	missing: m.ls_ev_missing,
	transferred_out: m.ls_ev_transferred_out,
	sold: m.ls_ev_sold,
	reconciliation: m.ls_ev_reconciliation,
	health_observation: m.ls_ev_health_observation,
	treatment: m.ls_ev_treatment
};

export const eventLabel = (t: string) => eventLabels[t]?.() ?? t;
export const isObservation = (t: string) => (observationTypes as readonly string[]).includes(t);

/** "+3", "−1" (true minus sign) or "" for observations. */
export function deltaText(delta: number): string {
	if (delta === 0) return '';
	return delta > 0 ? `+${delta}` : `−${Math.abs(delta)}`;
}

/** Algae, pods and clean-up crews are counted roughly; the UI says so (domain review LS6). */
export function quantityIsApproximate(category: string): boolean {
	return category === 'algae' || category === 'invertebrate';
}

// --- Species --------------------------------------------------------------------------------

type Named = Pick<Species, 'common_name_vi' | 'common_name_en' | 'scientific_name'>;

/** The best name for the UI language, falling back to the other names. */
export function speciesName(s: Named | null | undefined, locale: string): string {
	if (!s) return '';
	const primary = locale.startsWith('vi') ? s.common_name_vi : s.common_name_en;
	const secondary = locale.startsWith('vi') ? s.common_name_en : s.common_name_vi;
	return primary ?? secondary ?? s.scientific_name ?? '';
}

// --- Entries --------------------------------------------------------------------------------

export function groupByCategory(
	entries: LivestockEntry[]
): [LivestockCategory, LivestockEntry[]][] {
	return livestockCategories
		.map(
			(c) => [c, entries.filter((e) => e.category === c)] as [LivestockCategory, LivestockEntry[]]
		)
		.filter(([, list]) => list.length > 0);
}

export type EntryForm = {
	species_id: string;
	category: LivestockCategory;
	display_name: string;
	/** Bound to `type="number"`: may be a number. */
	quantity: string | number;
	form: string;
	coral_type: string;
	quarantine_status: string;
	location_note: string;
	source_note: string;
	notes: string;
};

export function emptyEntryForm(): EntryForm {
	return {
		species_id: '',
		category: 'fish',
		display_name: '',
		quantity: '1',
		form: '',
		coral_type: '',
		quarantine_status: '',
		location_note: '',
		source_note: '',
		notes: ''
	};
}

export function fromEntry(e: LivestockEntry): EntryForm {
	return {
		species_id: e.species?.id ?? '',
		category: e.category,
		display_name: e.display_name,
		quantity: String(e.current_quantity),
		form: e.form ?? '',
		coral_type: e.coral_type ?? '',
		quarantine_status: e.quarantine_status ?? '',
		location_note: e.location_note ?? '',
		source_note: e.source_note ?? '',
		notes: e.notes ?? ''
	};
}

const opt = (s: string) => (s.trim() === '' ? null : s.trim());

function description(f: EntryForm) {
	return {
		species_id: f.species_id || null,
		category: f.category,
		display_name: f.display_name.trim(),
		form: (f.form || null) as LivestockEntryCreate['form'],
		// Only corals have a coral type; the API rejects it otherwise (`coral_only`).
		coral_type: (f.category === 'coral' && f.coral_type
			? f.coral_type
			: null) as LivestockEntryCreate['coral_type'],
		quarantine_status: (f.quarantine_status || null) as LivestockEntryCreate['quarantine_status'],
		location_note: opt(f.location_note),
		source_note: opt(f.source_note),
		notes: opt(f.notes)
	};
}

export function entryToCreate(f: EntryForm, addedAt: string): LivestockEntryCreate {
	const q = Number(String(f.quantity ?? '').trim());
	return {
		...description(f),
		quantity: Number.isInteger(q) ? q : 0,
		...(addedAt ? { added_at: addedAt } : {})
	};
}

/** Merge patch of the description only; the quantity changes through events. */
export function entryToPatch(f: EntryForm, original: LivestockEntry): LivestockEntryPatch {
	const next = description(f);
	const before = description(fromEntry(original));
	const patch: Record<string, unknown> = { version: original.version };
	for (const key of Object.keys(next) as (keyof typeof next)[]) {
		if (next[key] !== before[key]) patch[key] = next[key];
	}
	return patch as LivestockEntryPatch;
}
