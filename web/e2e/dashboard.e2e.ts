import AxeBuilder from '@axe-core/playwright';
import { expect, test } from '@playwright/test';
import { storageState } from './global-setup';
import { t } from './helpers';

test.use({ storageState });

test('the tank dashboard summarises what needs attention and recent activity', async ({ page }) => {
	await page.goto('/tanks/new');
	await page.getByLabel(t.field_name, { exact: true }).fill('Dashboard tank');
	await page.getByLabel(t.field_display_volume, { exact: true }).fill('90');
	await page.getByLabel(t.field_system_volume, { exact: true }).fill('110');
	await page.getByRole('button', { name: t.tank_create_submit }).click();
	await expect(page.getByRole('heading', { name: 'Dashboard tank', level: 1 })).toBeVisible();
	const tankURL = page.url();

	const attention = page.getByRole('region', { name: t.db_attention_title });
	await expect(attention.getByText(t.db_all_clear)).toBeVisible();
	const recent = page.getByRole('region', { name: t.db_recent_title });
	await expect(recent.getByText(t.tl_tank_created)).toBeVisible();
	const results = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag22aa'])
		.analyze();
	expect(results.violations.map((v) => `${v.id}: ${v.help}`)).toEqual([]);

	// A maintenance task due today shows up as needing attention, linked to its page.
	await page
		.getByRole('navigation', { name: t.tank_sections })
		.getByRole('link', { name: t.tank_section_maintenance })
		.click();
	await page.getByRole('link', { name: t.mt_plan_new }).click();
	await page.getByLabel(t.mt_title_label).fill('Clean skimmer cup');
	await page.getByRole('button', { name: t.mt_plan_create_submit }).click();
	await expect(
		page.getByRole('heading', { name: t.tank_section_maintenance, level: 1 })
	).toBeVisible();

	await page.goto(tankURL);
	await expect(attention.getByRole('link', { name: /1/ })).toHaveAttribute(
		'href',
		/\/maintenance$/
	);
});
