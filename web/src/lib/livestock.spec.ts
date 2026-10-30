import { describe, expect, it } from 'vitest';
import type { LivestockEntry } from '#lib/api/client.js';
import {
	deltaText,
	emptyEntryForm,
	entryToCreate,
	entryToPatch,
	fromEntry,
	groupByCategory,
	isObservation,
	speciesName
} from './livestock';

const clown: LivestockEntry = {
	id: 'l1',
	tank_id: 't1',
	species: {
		id: 's1',
		common_name_vi: 'Cá hề',
		common_name_en: 'Ocellaris clownfish',
		scientific_name: 'Amphiprion ocellaris'
	},
	category: 'fish',
	coral_type: null,
	display_name: 'Nemo pair',
	form: 'individual',
	quarantine_status: 'completed',
	location_note: null,
	source_note: null,
	notes: null,
	current_quantity: 2,
	present: true,
	added_at: '2026-10-01T00:00:00Z',
	created_at: '2026-10-01T00:00:00Z',
	updated_at: '2026-10-01T00:00:00Z',
	version: 3
};

describe('livestock helpers', () => {
	it('names species in the UI language with fallbacks', () => {
		expect(speciesName(clown.species, 'vi')).toBe('Cá hề');
		expect(speciesName(clown.species, 'en')).toBe('Ocellaris clownfish');
		expect(
			speciesName(
				{ common_name_vi: null, common_name_en: null, scientific_name: 'Acropora sp.' },
				'vi'
			)
		).toBe('Acropora sp.');
	});

	it('formats deltas and classifies events', () => {
		expect(deltaText(3)).toBe('+3');
		expect(deltaText(-1)).toBe('−1');
		expect(deltaText(0)).toBe('');
		expect(isObservation('treatment')).toBe(true);
		expect(isObservation('died')).toBe(false);
	});

	it('groups by category in a fixed order', () => {
		const coral = { ...clown, id: 'l2', category: 'coral' as const };
		expect(groupByCategory([coral, clown]).map(([c]) => c)).toEqual(['fish', 'coral']);
	});
});

describe('entry form', () => {
	it('creates with an integer quantity and drops the coral type for non-corals', () => {
		const f = emptyEntryForm();
		f.display_name = ' Clean-up crew ';
		f.category = 'invertebrate';
		f.coral_type = 'sps';
		f.quantity = 12; // type="number" binds a number
		expect(entryToCreate(f, '')).toMatchObject({
			display_name: 'Clean-up crew',
			category: 'invertebrate',
			coral_type: null,
			quantity: 12
		});
	});

	it('patches only the description, never the quantity', () => {
		const f = fromEntry(clown);
		f.quantity = '7';
		expect(entryToPatch(f, clown)).toEqual({ version: 3 });
		f.location_note = 'left rockwork';
		expect(entryToPatch(f, clown)).toEqual({ version: 3, location_note: 'left rockwork' });
	});
});
