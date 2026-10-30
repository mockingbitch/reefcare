import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Field from './Field.svelte';

describe('Field.svelte', () => {
	it('labels the input and links hint and error for screen readers', async () => {
		render(Field, {
			label: 'Tank name',
			name: 'name',
			hint: 'Shown to viewers',
			error: 'Required.'
		});
		const input = page.getByRole('textbox', { name: 'Tank name' });
		await expect.element(input).toBeInTheDocument();
		await expect.element(input).toHaveAttribute('aria-invalid', 'true');
		await expect.element(input).toHaveAccessibleDescription('Shown to viewers Required.');
	});

	it('marks nothing invalid without an error', async () => {
		render(Field, { label: 'Salt mix', name: 'salt_mix', optional: true });
		const input = page.getByRole('textbox', { name: /Salt mix/ });
		await expect.element(input).not.toHaveAttribute('aria-invalid');
	});
});
