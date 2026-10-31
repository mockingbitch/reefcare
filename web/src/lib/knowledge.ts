// Knowledge base helpers (api/openapi.yaml Knowledge*). The editor edits the structured
// lists as plain text, one item per line: "key | text", recommendations "key | claim | text".
import type { ClaimClass, KnowledgeStructured } from '#lib/api/client.js';
import { m } from '#lib/paraglide/messages.js';

export const claimClasses: ClaimClass[] = ['established', 'expert', 'community', 'unverified'];
export const categories = [
	'water_chemistry',
	'testing',
	'dosing',
	'livestock_health',
	'equipment',
	'maintenance',
	'general'
] as const;

export function claimLabel(c: string): string {
	switch (c) {
		case 'established':
			return m.kb_claim_established();
		case 'expert':
			return m.kb_claim_expert();
		case 'community':
			return m.kb_claim_community();
		case 'unverified':
			return m.kb_claim_unverified();
		default:
			return c;
	}
}

export function statusLabel(s: string): string {
	switch (s) {
		case 'draft':
			return m.kb_status_draft();
		case 'in_review':
			return m.kb_status_in_review();
		case 'published':
			return m.kb_status_published();
		case 'retired':
			return m.kb_status_retired();
		default:
			return s;
	}
}

export function categoryLabel(c: string): string {
	const labels: Record<string, () => string> = {
		water_chemistry: m.kb_cat_water_chemistry,
		testing: m.kb_cat_testing,
		dosing: m.kb_cat_dosing,
		livestock_health: m.kb_cat_livestock_health,
		equipment: m.kb_cat_equipment,
		maintenance: m.kb_cat_maintenance,
		general: m.kb_cat_general
	};
	return labels[c]?.() ?? c;
}

type Item = { key: string; text: string };
type Rec = { key: string; text: string; claim_class: ClaimClass };

/** Lists as editable text (one item per line). */
export type StructuredText = {
	verifications: string;
	recommendations: string;
	contraindications: string;
	safety_warnings: string;
};

const itemsToText = (items: Item[] | undefined) =>
	(items ?? []).map((i) => `${i.key} | ${i.text}`).join('\n');

export function structuredToText(s: KnowledgeStructured | undefined): StructuredText {
	return {
		verifications: itemsToText(s?.verifications),
		recommendations: (s?.recommendations ?? [])
			.map((r) => `${r.key} | ${r.claim_class} | ${r.text}`)
			.join('\n'),
		contraindications: itemsToText(s?.contraindications),
		safety_warnings: itemsToText(s?.safety_warnings)
	};
}

function lines(text: string): string[][] {
	return text
		.split('\n')
		.map((l) => l.trim())
		.filter((l) => l !== '')
		.map((l) => l.split('|').map((p) => p.trim()));
}

/**
 * Parses the editable text back. Lines that don't have the expected parts are reported (1-based
 * line numbers per list) instead of being dropped; the server validates keys and texts.
 */
export function textToStructured(t: StructuredText): {
	structured: KnowledgeStructured;
	errors: string[];
} {
	const errors: string[] = [];
	const items = (name: keyof StructuredText): Item[] =>
		lines(t[name]).flatMap((parts, i) => {
			if (parts.length < 2) {
				errors.push(`${name}:${i + 1}`);
				return [];
			}
			return [{ key: parts[0], text: parts.slice(1).join(' | ') }];
		});
	const verifications = items('verifications');
	const recommendations: Rec[] = lines(t.recommendations).flatMap((parts, i) => {
		if (parts.length < 3 || !claimClasses.includes(parts[1] as ClaimClass)) {
			errors.push(`recommendations:${i + 1}`);
			return [];
		}
		return [
			{ key: parts[0], claim_class: parts[1] as ClaimClass, text: parts.slice(2).join(' | ') }
		];
	});
	return {
		structured: {
			verifications,
			recommendations,
			contraindications: items('contraindications'),
			safety_warnings: items('safety_warnings')
		},
		errors
	};
}

/** Comma- or space-separated tags to a clean list. */
export function tags(s: string): string[] {
	return s
		.split(/[\s,]+/)
		.map((t) => t.trim())
		.filter((t) => t !== '');
}

/** Only https links are rendered as links (the server accepts nothing else). */
export function safeHref(url: string | null | undefined): string | null {
	return url && url.startsWith('https://') ? url : null;
}
