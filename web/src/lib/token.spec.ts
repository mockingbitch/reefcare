import { describe, expect, it } from 'vitest';
import { csrfToken } from '#lib/api/client.js';
import { parseFragmentToken, safeNext } from './token';

describe('parseFragmentToken', () => {
	it('reads #token=', () => {
		expect(parseFragmentToken('#token=abcDEF123_-abcDEF123_-')).toBe('abcDEF123_-abcDEF123_-');
	});
	it('rejects missing or malformed tokens', () => {
		expect(parseFragmentToken('')).toBeNull();
		expect(parseFragmentToken('#token=')).toBeNull();
		expect(parseFragmentToken('#token=<script>')).toBeNull();
	});
});

describe('safeNext', () => {
	const origin = 'https://reefcare.example';
	it('allows same-origin paths', () => {
		expect(safeNext('/tanks/1?x=1#y', origin)).toBe('/tanks/1?x=1#y');
	});
	it('rejects other origins, including ones the URL parser would create', () => {
		for (const next of [
			'//evil.example',
			'/\\evil.example',
			'https://evil.example',
			'/\t/evil.example', // what ?next=/%09/evil.example decodes to
			'/\n/evil.example',
			'/\r\\evil.example',
			'tanks',
			''
		]) {
			expect(safeNext(next, origin), JSON.stringify(next)).toBeNull();
		}
		expect(safeNext(null, origin)).toBeNull();
	});
});

describe('csrfToken', () => {
	it('reads the secure or development cookie', () => {
		expect(csrfToken('a=1; __Host-rc_csrf=tok%3D; b=2')).toBe('tok=');
		expect(csrfToken('rc_csrf=dev')).toBe('dev');
		expect(csrfToken('rc_session=x')).toBe('');
	});
});
