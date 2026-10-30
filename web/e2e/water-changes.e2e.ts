import AxeBuilder from '@axe-core/playwright';
import { expect, test, type Page } from '@playwright/test';
import { signUp, t, uniqueEmail } from './helpers';

async function expectAccessible(page: Page) {
	const results = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag22aa'])
		.analyze();
	expect(results.violations.map((v) => `${v.id}: ${v.help}`)).toEqual([]);
}

async function createTank(page: Page, name: string): Promise<string> {
	await page.goto('/tanks/new');
	await page.getByLabel(t.field_name, { exact: true }).fill(name);
	await page.getByLabel(t.field_display_volume, { exact: true }).fill('60');
	await page.getByLabel(t.field_system_volume, { exact: true }).fill('72');
	await page.getByRole('button', { name: t.tank_create_submit }).click();
	await expect(page.getByRole('heading', { name, level: 1 })).toBeVisible();
	return page.url().split('/tanks/')[1];
}

test('schedule, record and complete water changes', async ({ page }) => {
	test.setTimeout(120_000);
	await signUp(page, uniqueEmail('wc'), 'Lan');
	await createTank(page, 'Nano SPS');

	await page.getByRole('link', { name: t.tank_section_water_changes }).click();
	await expect(page.getByRole('heading', { name: t.wc_title, level: 1 })).toBeVisible();
	await expect(page.getByText(t.wc_schedules_empty)).toBeVisible();
	await expectAccessible(page);

	// New schedule: the preview lists the next three dates computed by the API.
	await page.getByRole('link', { name: t.wc_schedule_new }).click();
	await page.getByLabel(t.wc_planned_volume_label).fill('10,5');
	const preview = page.getByRole('heading', { name: t.rec_preview_title }).locator('..');
	await expect(preview.getByRole('listitem')).toHaveCount(3);
	await expectAccessible(page);
	await page.getByRole('button', { name: t.wc_schedule_create_submit }).click();

	await expect(page.getByRole('heading', { name: t.wc_title, level: 1 })).toBeVisible();
	const schedules = page.getByRole('region', { name: t.wc_schedules_title });
	await expect(schedules.getByText(/10,5 L/)).toBeVisible();
	const upcoming = page.getByRole('region', { name: t.wc_upcoming_title });
	await expect(upcoming.getByRole('button', { name: t.wc_complete }).first()).toBeVisible();

	// Record a change done without a schedule; the estimate and its caveats are shown.
	await page.getByRole('link', { name: t.wc_record }).click();
	await page.getByLabel(t.wc_actual_volume_label).fill('8');
	await page.getByRole('button', { name: t.save }).click();
	const history = page.getByRole('region', { name: t.wc_history_title });
	await expect(history.getByText(/Đã thay 8 L/)).toBeVisible();
	await expect(history.getByText(/tổng thể tích hệ thống/).first()).toBeVisible();

	// Complete the next planned change from its dialog.
	await upcoming.getByRole('button', { name: t.wc_complete }).first().click();
	const dialog = page.getByRole('dialog', { name: t.wc_complete_title });
	await expect(dialog.getByLabel(t.wc_actual_volume_label)).toHaveValue('10.5');
	await expectAccessible(page);
	await dialog.getByRole('button', { name: t.wc_complete }).click();
	await expect(dialog).toBeHidden();
	await expect(history.getByText(/Đã thay 10,5 L/)).toBeVisible();
});
