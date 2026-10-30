import { describe, expect, it } from 'vitest';
import type { Tank } from '#lib/api/client.js';
import {
	clientProblems,
	decimalInput,
	emptyTankForm,
	fromTank,
	systemBelowDisplay,
	toCreate,
	toPatch
} from './tank-form';

const tank: Tank = {
	id: '0192f0c4-0000-7000-8000-000000000001',
	owner_id: '0192f0c4-0000-7000-8000-000000000002',
	access: 'owner',
	name: 'Nano',
	description: null,
	tank_type: 'sps_dominant',
	status: 'active',
	dimensions_cm: { length: '60', width: '40', height: '35' },
	display_volume_l: '60',
	sump_volume_l: '25',
	system_volume_l: '72',
	setup_date: '2025-06-01',
	water_source: null,
	salt_mix: 'Red Sea',
	archived_at: null,
	created_at: '2026-10-29T03:00:00Z',
	updated_at: '2026-10-29T03:00:00Z',
	version: 3
};

describe('decimalInput', () => {
	it('accepts a comma as decimal separator and trims', () => {
		expect(decimalInput(' 60,5 ')).toBe('60.5');
		expect(decimalInput('72')).toBe('72');
	});
	it('leaves anything else for the API to judge', () => {
		expect(decimalInput('1,000.5')).toBe('1,000.5');
		expect(decimalInput('abc')).toBe('abc');
	});
});

describe('toCreate', () => {
	it('sends decimals as strings and omits empty optionals as null', () => {
		const f = { ...emptyTankForm(), name: ' QT ', display_volume_l: '40', system_volume_l: '40,5' };
		expect(toCreate(f)).toEqual({
			name: 'QT',
			description: null,
			tank_type: 'mixed_reef',
			status: 'cycling',
			dimensions_cm: null,
			display_volume_l: '40',
			sump_volume_l: null,
			system_volume_l: '40.5',
			setup_date: null,
			salt_mix: null,
			water_source: null
		});
	});
});

describe('clientProblems', () => {
	it('requires all three dimensions or none', () => {
		expect(clientProblems({ ...emptyTankForm(), length: '60' })).toEqual({
			dimensions_cm: 'dimensions_all_or_none'
		});
		expect(clientProblems({ ...emptyTankForm(), length: '1', width: '2', height: '3' })).toEqual(
			{}
		);
		expect(clientProblems(emptyTankForm())).toEqual({});
	});
});

describe('systemBelowDisplay', () => {
	it('warns only when both are numbers and system < display', () => {
		const f = emptyTankForm();
		expect(systemBelowDisplay({ ...f, display_volume_l: '60', system_volume_l: '50' })).toBe(true);
		expect(systemBelowDisplay({ ...f, display_volume_l: '60', system_volume_l: '72' })).toBe(false);
		expect(systemBelowDisplay({ ...f, display_volume_l: '60', system_volume_l: '' })).toBe(false);
	});
});

describe('toPatch', () => {
	it('contains only changed fields plus version', () => {
		const f = fromTank(tank);
		expect(toPatch(f, tank)).toEqual({ version: 3 });
		f.name = 'Nano SPS';
		f.sump_volume_l = '';
		f.length = f.width = f.height = '';
		expect(toPatch(f, tank)).toEqual({
			version: 3,
			name: 'Nano SPS',
			sump_volume_l: null,
			dimensions_cm: null
		});
	});
	it('sends dimensions as a whole object when one member changes', () => {
		const f = fromTank(tank);
		f.length = '90';
		expect(toPatch(f, tank)).toEqual({
			version: 3,
			dimensions_cm: { length: '90', width: '40', height: '35' }
		});
	});
});
