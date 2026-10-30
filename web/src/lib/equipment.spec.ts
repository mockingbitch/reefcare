import { describe, expect, it } from 'vitest';
import type { Equipment, MaintenancePlan } from '#lib/api/client.js';
import {
	diffSettings,
	emptyMaintenanceForm,
	equipmentToCreate,
	equipmentToPatch,
	fromEquipment,
	fromMaintenancePlan,
	maintenanceToCreate,
	maintenanceToPatch,
	rowsToSettings,
	settingsToRows,
	warrantyExpired
} from './equipment';

const device: Equipment = {
	id: 'e1',
	tank_id: 't1',
	category: 'lighting',
	name: 'Radion',
	manufacturer: 'EcoTech',
	model: null,
	serial_number: null,
	purchased_on: '2025-01-10',
	warranty_until: null,
	installed_at: '2025-01-11T02:00:00Z',
	removed_at: null,
	removal_reason: null,
	notes: null,
	configuration: null,
	created_at: '2025-01-11T02:00:00Z',
	updated_at: '2025-01-11T02:00:00Z',
	version: 4
};

describe('settings rows', () => {
	it('round-trips typed values and drops blank keys', () => {
		const rows = settingsToRows({ photoperiod_h: 8, schedule: 'AB+', moonlight: false });
		expect(rows.map((r) => r.kind)).toEqual(['number', 'text', 'boolean']);
		rows.push({ key: '  ', kind: 'text', value: 'ignored' });
		expect(rowsToSettings(rows)).toEqual({ photoperiod_h: 8, schedule: 'AB+', moonlight: false });
	});

	it('accepts a decimal comma and keeps unparseable numbers as text for the API', () => {
		expect(
			rowsToSettings([
				{ key: 'flow', kind: 'number', value: '42,5' },
				{ key: 'bad', kind: 'number', value: 'fast' }
			])
		).toEqual({ flow: 42.5, bad: 'fast' });
	});

	it('diffs versions', () => {
		expect(diffSettings({ a: 1, b: 'x', c: true }, { a: 2, b: 'x', d: 'new' })).toEqual({
			added: ['d'],
			removed: ['c'],
			changed: ['a']
		});
		expect(diffSettings(null, { a: 1 }).added).toEqual(['a']);
	});
});

describe('equipment form', () => {
	it('creates with nulls for blanks and patches only changes', () => {
		const f = fromEquipment(device);
		expect(equipmentToCreate(f, '2025-01-11T02:00:00Z')).toMatchObject({
			name: 'Radion',
			model: null,
			installed_at: '2025-01-11T02:00:00Z'
		});
		expect(equipmentToPatch(f, device)).toEqual({ version: 4 });
		f.model = ' XR15 G6 ';
		expect(equipmentToPatch(f, device)).toEqual({ version: 4, model: 'XR15 G6' });
	});

	it('knows when a warranty has expired', () => {
		expect(warrantyExpired('2026-10-30', '2026-10-31')).toBe(true);
		expect(warrantyExpired('2026-10-31', '2026-10-31')).toBe(false);
		expect(warrantyExpired(null, '2026-10-31')).toBe(false);
	});
});

describe('maintenance plan form', () => {
	const plan: MaintenancePlan = {
		id: 'm1',
		tank_id: 't1',
		equipment_id: 'e1',
		equipment_name: 'Radion',
		title: 'Clean lens',
		interval_days: 30,
		timezone: 'Asia/Ho_Chi_Minh',
		starts_on: '2026-10-01',
		last_performed_at: null,
		next_due_on: '2026-10-01',
		active: true,
		overdue: true,
		due_today: false,
		notes: null,
		created_at: '2026-10-01T00:00:00Z',
		updated_at: '2026-10-01T00:00:00Z',
		version: 1
	};

	it('creates and patches', () => {
		const f = emptyMaintenanceForm('2026-11-01', 'Asia/Ho_Chi_Minh');
		f.title = ' Replace carbon ';
		f.interval_days = '14';
		expect(maintenanceToCreate(f)).toEqual({
			title: 'Replace carbon',
			interval_days: 14,
			timezone: 'Asia/Ho_Chi_Minh',
			starts_on: '2026-11-01',
			equipment_id: null,
			notes: null
		});
		const edit = fromMaintenancePlan(plan);
		edit.interval_days = '21';
		expect(maintenanceToPatch(edit, plan)).toEqual({ version: 1, interval_days: 21 });
	});
});

describe('number inputs', () => {
	it('accepts the number a type="number" input binds', () => {
		const f = emptyMaintenanceForm('2026-11-01', 'UTC');
		f.title = 'Clean lens';
		f.interval_days = 30;
		expect(maintenanceToCreate(f).interval_days).toBe(30);
	});
});
