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

const fill = (s: string, vars: Record<string, string>) =>
	s.replace(/\{(\w+)\}/g, (_, k: string) => vars[k] ?? '');

test('water tests: record, unit-mistake warning, correction, targets and trends', async ({
	page
}) => {
	test.setTimeout(120_000);

	await page.goto('/tanks/new');
	await page.getByLabel(t.field_name, { exact: true }).fill('Reef 90');
	await page.getByLabel(t.field_display_volume, { exact: true }).fill('90');
	await page.getByLabel(t.field_system_volume, { exact: true }).fill('110');
	await page.getByRole('button', { name: t.tank_create_submit }).click();
	await page
		.getByRole('navigation', { name: t.tank_sections })
		.getByRole('link', { name: t.wp_title })
		.click();
	await expect(page.getByText(t.wp_empty)).toBeVisible();
	await expectAccessible(page);

	// Targets first, so the test below is compared with them.
	await page.getByRole('link', { name: t.wp_targets }).click();
	const alk = page.getByRole('group', {
		name: new RegExp(t.wp_p_alkalinity.replace(/[()]/g, '\\$&'))
	});
	await alk.getByLabel(t.wp_min).fill('7,5');
	await alk.getByLabel(t.wp_max).fill('9');
	await expectAccessible(page);
	await page.getByRole('button', { name: t.save }).click();
	await expect(page.getByText(t.wp_targets_saved)).toBeVisible();
	await page.getByRole('link', { name: `← ${t.wp_title}` }).click();

	// Alkalinity in meq/L, and a specific gravity typed into the ppt field.
	await page.getByRole('link', { name: t.wp_record }).click();
	await page.getByLabel(t.wp_p_alkalinity, { exact: true }).fill('3');
	await page
		.getByLabel(fill(t.wp_unit_for, { parameter: t.wp_p_alkalinity }))
		.selectOption('meq/L');
	await page.getByLabel(t.wp_p_salinity, { exact: true }).fill('1,025');
	await expectAccessible(page);
	await page.getByRole('button', { name: fill(t.wp_save, { count: '2' }) }).click();
	await expect(page.getByText(t.wp_saved_check)).toBeVisible();
	await expect(page.getByText(t.wp_warn_sg, { exact: false })).toBeVisible();
	await page.getByRole('link', { name: t.wp_to_history }).click();

	// Latest values: converted, compared with the target in words.
	const latest = page.getByRole('region', { name: t.wp_latest });
	await expect(latest.getByText('8,41 dKH')).toBeVisible();
	await expect(latest.getByText(`✓ ${t.wp_status_within}`)).toBeVisible();

	// Correct the salinity: the old value stays in the history with the reason.
	const history = page.getByRole('region', { name: t.wp_history });
	await history
		.getByRole('button', { name: fill(t.wp_correct_label, { parameter: t.wp_p_salinity }) })
		.click();
	const dialog = page.getByRole('dialog');
	await dialog.getByLabel(t.wp_unit).selectOption('SG');
	await dialog.getByLabel(t.wp_reason).fill('Nhập nhầm đơn vị');
	await expectAccessible(page);
	await dialog.getByRole('button', { name: t.wp_correct_save }).click();
	await expect(dialog).toBeHidden();
	await expect(history.getByText(/Nhập nhầm đơn vị/)).toBeVisible();
	await expect(history.getByText(`≈ ${t.wp_estimate}`)).toBeVisible();
	await expect(latest.getByText('33,2 ppt')).toBeVisible();

	// Trends: one chart per parameter with a text summary and a table.
	await page.getByRole('link', { name: t.wp_trends }).click();
	const chart = page.getByRole('figure', {
		name: new RegExp(t.wp_p_alkalinity.replace(/[()]/g, '\\$&'))
	});
	await expect(chart.getByText(/1 kết quả/)).toBeVisible();
	await chart.getByText(t.wp_chart_table).click();
	await expect(chart.getByRole('cell', { name: '8,41 dKH' })).toBeVisible();
	await expectAccessible(page);

	// The dashboard shows the latest values.
	await page.getByRole('link', { name: `← ${t.wp_title}` }).click();
	await page.getByRole('link', { name: '← Reef 90' }).click();
	const water = page.getByRole('region', { name: t.wp_title });
	await expect(water.getByText(/8,41 dKH/)).toBeVisible();
});
