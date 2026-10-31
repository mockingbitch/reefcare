import AxeBuilder from '@axe-core/playwright';
import { expect, test, type Page } from '@playwright/test';
import { storageState } from './global-setup';
import { t } from './helpers';

test.use({ storageState });

async function expectAccessible(page: Page) {
	const results = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag22aa'])
		.analyze();
	expect(results.violations.map((v) => `${v.id}: ${v.help}`)).toEqual([]);
}

// The rules themselves are tested in internal/intelligence (fixtures + golden files); this
// covers the pages: run an assessment, read its findings, rate it and retract it.
test('assessments: run, read findings, feedback and retraction', async ({ page }) => {
	test.setTimeout(120_000);
	await page.goto('/tanks/new');
	await page.getByLabel(t.field_name, { exact: true }).fill('Assess 60');
	await page.getByLabel(t.field_display_volume, { exact: true }).fill('60');
	await page.getByLabel(t.field_system_volume, { exact: true }).fill('72');
	await page.getByRole('button', { name: t.tank_create_submit }).click();
	await page
		.getByRole('navigation', { name: t.tank_sections })
		.getByRole('link', { name: t.ia_title })
		.click();
	await expect(page.getByRole('heading', { name: t.ia_title, level: 1 })).toBeVisible();
	await expect(page.getByText(t.ia_empty)).toBeVisible();
	await expectAccessible(page);

	// A new tank has nothing measured: the assessment lists what to test.
	await page.getByRole('button', { name: t.ia_run }).click();
	await expect(page.getByText(t.ia_conf_basis)).toBeVisible();
	await expect(page.getByRole('heading', { name: /thông số chưa đo hoặc đã cũ/ })).toBeVisible();
	await expect(page.getByText(t.ia_provisional)).toBeVisible();
	await expectAccessible(page);

	await page.getByLabel(t.ia_rating_useful, { exact: true }).check();
	await page.getByRole('button', { name: t.ia_send_feedback }).click();
	await expect(page.getByText(t.ia_feedback_saved)).toBeVisible();

	await page.getByRole('button', { name: t.ia_retract }).click();
	const dialog = page.getByRole('dialog');
	await dialog.getByLabel(t.ia_retract_reason).fill('Tested by mistake');
	await dialog.getByRole('button', { name: t.ia_retract }).click();
	await expect(page.getByText(/Tested by mistake/)).toBeVisible();
	await expect(page.getByRole('button', { name: t.ia_retract })).toHaveCount(0);

	await page.getByRole('link', { name: new RegExp(t.ia_back) }).click();
	await expect(page.getByText(t.ia_retracted, { exact: true })).toBeVisible();
});
