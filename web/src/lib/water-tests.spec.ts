import { describe, expect, it } from 'vitest';
import type { LatestReading, ParameterType } from '#lib/api/client.js';
import {
	ageText,
	emptyRows,
	freshnessText,
	niceTicks,
	rowErrors,
	scale,
	targetRowErrors,
	targetRows,
	targetsToReplace,
	targetText,
	testToCreate,
	valueDomain,
	valueText
} from './water-tests';

const param = (code: string, canonical: string, units: string[], hours = 168): ParameterType => ({
	code,
	canonical_unit: canonical,
	units: units.map((u) => ({ code: u, estimate: u === 'SG' })),
	hard_min: '0',
	hard_max: '100',
	freshness_hours: hours,
	decimals: 2
});
const catalog = [
	param('alkalinity', 'dKH', ['dKH', 'meq/L', 'ppm CaCO3']),
	param('ph', 'pH', ['pH']),
	param('salinity', 'ppt', ['ppt', 'PSU', 'SG'])
];

describe('entry form', () => {
	it('starts every parameter in the preferred unit when the catalog accepts it', () => {
		const rows = emptyRows(catalog, { alkalinity: 'meq/L', salinity: 'gal' });
		expect(rows.map((r) => r.unit)).toEqual(['meq/L', 'pH', 'ppt']);
	});

	it('sends only filled rows and maps error indexes back to form rows', () => {
		const rows = emptyRows(catalog);
		rows[1].value = 8.1; // number inputs bind as numbers
		rows[2].value = '35,2';
		rows[2].method = 'refractometer';
		rows[2].testKit = '  Hanna  ';
		const { body, rowIndex } = testToCreate(rows, '2026-10-30T01:00:00Z', ' ');
		expect(body).toEqual({
			measured_at: '2026-10-30T01:00:00Z',
			notes: null,
			readings: [
				{ parameter: 'ph', value: '8.1', unit: 'pH', method: null, test_kit: null },
				{
					parameter: 'salinity',
					value: '35.2',
					unit: 'ppt',
					method: 'refractometer',
					test_kit: 'Hanna'
				}
			]
		});
		expect(rowIndex).toEqual([1, 2]);
		expect(rowErrors({ 'readings[1].value': 'bad', measured_at: 'future' }, rowIndex)).toEqual({
			'2.value': 'bad',
			measured_at: 'future'
		});
	});
});

describe('targets', () => {
	it('round-trips current targets and drops empty rows', () => {
		const rows = targetRows(catalog, [
			{
				parameter: 'alkalinity',
				min: '7.5',
				ideal: null,
				max: '9',
				freshness_hours: 72,
				valid_from: '2026-10-01T00:00:00Z',
				valid_to: null
			}
		]);
		expect(rows[0]).toEqual({
			parameter: 'alkalinity',
			min: '7.5',
			ideal: '',
			max: '9',
			freshnessDays: '3'
		});
		rows[2].max = '36';
		const { body, rowIndex } = targetsToReplace(rows);
		expect(body.targets).toEqual([
			{ parameter: 'alkalinity', min: '7.5', ideal: null, max: '9', freshness_hours: 72 },
			{ parameter: 'salinity', min: null, ideal: null, max: '36', freshness_hours: null }
		]);
		expect(rowIndex).toEqual([0, 2]);
		expect(
			targetRowErrors(
				{ 'targets[1].max': 'below', 'targets[0].freshness_hours': 'range', 'targets[1]': 'req' },
				rowIndex
			)
		).toEqual({ '2.max': 'below', '0.freshnessDays': 'range', '2.min': 'req' });
	});

	it('describes ranges with the locale separator', () => {
		expect(targetText({ min: '7.5', max: '9' }, 'dKH', 'vi')).toBe('7,5–9 dKH');
		expect(targetText({ min: null, max: '10' }, 'mg/L NO3', 'en')).toBe('≤ 10 mg/L NO3');
		expect(targetText({ min: '7.8', max: null }, 'pH', 'en')).toBe('≥ 7.8');
		expect(targetText({ min: null, max: null }, 'dKH', 'en')).toBe('');
	});
});

describe('display', () => {
	it('formats values with their unit', () => {
		expect(valueText('25.5', 'C', 'vi')).toBe('25,5 °C');
		expect(valueText('8.1', 'pH', 'en')).toBe('8.1');
	});

	it('states age and staleness in words', () => {
		const now = Date.parse('2026-10-30T12:00:00Z');
		expect(ageText('2026-10-30T09:00:00Z', 'en', now)).toBe('3 hours ago');
		expect(ageText('2026-10-21T12:00:00Z', 'en', now)).toBe('9 days ago');
		const l = (reading: string | null, stale: boolean) =>
			({
				parameter: 'alkalinity',
				canonical_unit: 'dKH',
				reading: reading ? ({ measured_at: reading } as LatestReading['reading']) : null,
				freshness_hours: 168,
				stale,
				target: null
			}) as LatestReading;
		expect(freshnessText(l(null, true), 'en', now)).toBe('Chưa đo'); // messages render in the default locale (vi);
		expect(freshnessText(l('2026-10-21T12:00:00Z', true), 'en', now)).toBe('Đã cũ: 9 days ago');
	});
});

describe('chart geometry', () => {
	it('picks nice ticks that cover the domain', () => {
		expect(niceTicks(7.2, 8.9)).toEqual([7, 7.5, 8, 8.5, 9]);
		expect(niceTicks(0, 0.03)).toEqual([0, 0.01, 0.02, 0.03]);
		const flat = niceTicks(420, 420);
		expect(flat[0]).toBeLessThan(420);
		expect(flat.at(-1)).toBeGreaterThan(420);
	});

	it('scales linearly and includes target bounds in the domain', () => {
		const y = scale(0, 10, 200, 0);
		expect(y(0)).toBe(200);
		expect(y(5)).toBe(100);
		expect(valueDomain([8, 8.2], [{ min: '7.5', max: '9' }])).toEqual([7.5, 9]);
		expect(valueDomain([], [])).toEqual([0, 1]);
	});
});
