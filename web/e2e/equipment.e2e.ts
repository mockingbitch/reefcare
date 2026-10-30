import AxeBuilder from '@axe-core/playwright';
import { expect, test, type Page } from '@playwright/test';
import { storageState } from './global-setup';
import { t } from './helpers';

// Signed in as the shared keeper account; each test creates its own tank.
test.use({ storageState });

async function expectAccessible(page: Page) {
	const results = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag22aa'])
		.analyze();
	expect(results.violations.map((v) => `${v.id}: ${v.help}`)).toEqual([]);
}

test('devices, configuration versions and maintenance', async ({ page }) => {
	test.setTimeout(120_000);

	await page.goto('/tanks/new');
	await page.getByLabel(t.field_name, { exact: true }).fill('Frag tank');
	await page.getByLabel(t.field_display_volume, { exact: true }).fill('120');
	await page.getByLabel(t.field_system_volume, { exact: true }).fill('150');
	await page.getByRole('button', { name: t.tank_create_submit }).click();
	await page
		.getByRole('navigation', { name: t.tank_sections })
		.getByRole('link', { name: t.eq_title, exact: true })
		.click();
	await expect(page.getByText(t.eq_empty)).toBeVisible();

	// Add a light; the device page opens.
	await page.getByRole('link', { name: t.eq_add }).click();
	await page.getByLabel(t.eq_name_label, { exact: true }).fill('Radion XR15');
	await page.getByLabel(t.eq_manufacturer_label).fill('EcoTech');
	await expectAccessible(page);
	await page.getByRole('button', { name: t.eq_add }).click();
	await expect(page.getByRole('heading', { name: 'Radion XR15', level: 1 })).toBeVisible();

	// Two configuration versions; the history shows what changed.
	await page.getByRole('button', { name: t.eq_config_new }).click();
	await page.getByLabel(t.eq_setting_key).first().fill('photoperiod_h');
	await page.getByLabel(t.eq_setting_kind).first().selectOption('number');
	await page.getByLabel(t.eq_setting_value).first().fill('8');
	await expectAccessible(page);
	await page.getByRole('button', { name: t.eq_config_save }).click();
	const config = page.getByRole('region', { name: t.eq_config_title });
	await expect(config.getByText('photoperiod_h')).toBeVisible();

	await page.getByRole('button', { name: t.eq_config_new }).click();
	await page.getByLabel(t.eq_setting_value).first().fill('9,5');
	await page.getByRole('button', { name: t.eq_config_save }).click();
	await expect(config.getByText(/9[.,]5/)).toBeVisible();
	await config.getByText(t.eq_config_history, { exact: false }).click();
	await expect(config.getByText(/photoperiod_h/).last()).toBeVisible();

	// A maintenance plan for this device, due today, then marked done.
	await page.getByRole('link', { name: t.mt_plan_new }).click();
	await page.getByLabel(t.mt_title_label).fill('Clean lens');
	await page.getByLabel(t.mt_interval_label).fill('30');
	await expect(page.getByLabel(t.mt_device_label)).toHaveValue(/.+/);
	await page.getByRole('button', { name: t.mt_plan_create_submit }).click();
	const due = page.getByRole('region', { name: new RegExp(t.mt_due_title) });
	await expect(due.getByText('Clean lens')).toBeVisible();
	await expectAccessible(page);
	await due.getByRole('button', { name: t.mt_done }).click();
	await page.getByRole('dialog').getByRole('button', { name: t.mt_done }).click();
	const history = page.getByRole('region', { name: t.wc_history_title });
	await expect(history.getByText('Clean lens')).toBeVisible();
	await expect(due).toBeHidden();

	// Removing the device keeps it in history.
	await page.getByRole('link', { name: t.eq_title, exact: true }).click();
	await page.getByRole('link', { name: 'Radion XR15' }).click();
	await page.getByRole('button', { name: t.eq_remove }).click();
	await page.getByRole('dialog').getByRole('button', { name: t.eq_remove }).click();
	await expect(page.getByText(t.eq_removed_badge).first()).toBeVisible();
	await expect(page.getByRole('button', { name: t.eq_config_new })).toHaveCount(0);
});
