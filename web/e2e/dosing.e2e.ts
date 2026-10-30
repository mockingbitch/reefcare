import AxeBuilder from '@axe-core/playwright';
import { expect, test, type Page } from '@playwright/test';
import { signUp, t, uniqueEmail } from './helpers';

async function expectAccessible(page: Page) {
	const results = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag22aa'])
		.analyze();
	expect(results.violations.map((v) => `${v.id}: ${v.help}`)).toEqual([]);
}

test('products, a daily plan, confirming a dose and an unplanned dose', async ({ page }) => {
	test.setTimeout(120_000);
	await signUp(page, uniqueEmail('dose'), 'Lan');

	await page.goto('/tanks/new');
	await page.getByLabel(t.field_name, { exact: true }).fill('Mixed reef');
	await page.getByLabel(t.field_display_volume, { exact: true }).fill('200');
	await page.getByLabel(t.field_system_volume, { exact: true }).fill('250');
	await page.getByRole('button', { name: t.tank_create_submit }).click();
	await page.getByRole('link', { name: t.tank_section_dosing }).click();
	await expect(page.getByRole('heading', { name: t.dose_title, level: 1 })).toBeVisible();
	await expectAccessible(page);

	// A private product; ReefCare records amounts only.
	await page.getByRole('link', { name: t.dose_products_manage }).click();
	await page.getByLabel(t.dose_product_name).fill('Alk part A');
	await page.getByRole('button', { name: t.dose_product_create }).click();
	await expect(page.getByText(/Alk part A/).first()).toBeVisible();
	await expectAccessible(page);
	await page.goBack();

	// A daily plan of 10 ml with a preview of the next dates.
	await page.getByRole('link', { name: t.dose_plan_new }).click();
	await page.getByLabel(t.dose_product_label).selectOption({ label: 'Alk part A' });
	await page.getByLabel(t.dose_amount_label, { exact: true }).fill('10');
	await expect(
		page.getByRole('heading', { name: t.rec_preview_title }).locator('..').getByRole('listitem')
	).toHaveCount(3);
	await page.getByRole('button', { name: t.dose_plan_create_submit }).click();
	const plans = page.getByRole('region', { name: t.dose_plans_title });
	await expect(plans.getByText('Alk part A')).toBeVisible();

	// Confirm the next dose: one dose at a time, never "mark all". Doses more than 6 h away
	// are recorded through "Record early…" with a warning instead of the one-tap button.
	const upcoming = page.getByRole('region', { name: t.wc_upcoming_title });
	await expect(upcoming.getByRole('button', { name: /mark all|tất cả/i })).toHaveCount(0);
	await upcoming.getByRole('button').first().click();
	const dialog = page.getByRole('dialog');
	await expect(dialog.getByRole('heading', { name: /Alk part A/ })).toBeVisible();
	await expect(dialog.getByRole('radio', { name: t.dose_outcome_completed })).toBeChecked();
	await expectAccessible(page);
	await dialog.getByRole('button', { name: t.dose_record }).click();
	await expect(dialog).toBeHidden();
	const history = page.getByRole('region', { name: t.wc_history_title });
	await expect(history.getByText(t.dose_status_completed)).toBeVisible();

	// An unplanned dose far above the plan asks to check it first.
	await page.getByRole('link', { name: t.dose_record_ad_hoc }).click();
	await page.getByLabel(t.dose_product_label).selectOption({ label: 'Alk part A' });
	await page.getByLabel(t.dose_amount_label, { exact: true }).fill('50');
	await page.getByRole('button', { name: t.dose_record }).click();
	const check = page.getByRole('dialog', { name: t.dose_check_title });
	await expect(check.getByText(t.dose_check_over_plan)).toBeVisible();
	await check.getByRole('button', { name: t.dose_record }).click();
	await expect(page.getByRole('heading', { name: t.dose_title, level: 1 })).toBeVisible();
	await expect(history.getByText(t.dose_ad_hoc)).toBeVisible();
});
