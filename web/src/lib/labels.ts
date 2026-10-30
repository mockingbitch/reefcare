// Localized labels for API enum values.
import { m } from '#lib/paraglide/messages.js';

const tankTypes: Record<string, () => string> = {
	fish_only: m.tank_type_fish_only,
	fowlr: m.tank_type_fowlr,
	mixed_reef: m.tank_type_mixed_reef,
	sps_dominant: m.tank_type_sps_dominant,
	lps_dominant: m.tank_type_lps_dominant,
	soft_coral: m.tank_type_soft_coral,
	custom: m.tank_type_custom
};

const statuses: Record<string, () => string> = {
	cycling: m.status_cycling,
	active: m.status_active,
	paused: m.status_paused,
	archived: m.status_archived
};

const experience: Record<string, () => string> = {
	beginner: m.exp_beginner,
	intermediate: m.exp_intermediate,
	advanced: m.exp_advanced,
	professional: m.exp_professional
};

export const tankTypeLabel = (t: string) => tankTypes[t]?.() ?? t;
export const statusLabel = (s: string) => statuses[s]?.() ?? s;
export const experienceLabel = (e: string) => experience[e]?.() ?? e;
