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

test('livestock: add, events, a blocked negative count and reconciliation', async ({ page }) => {
	test.setTimeout(120_000);

	await page.goto('/tanks/new');
	await page.getByLabel(t.field_name, { exact: true }).fill('Nano');
	await page.getByLabel(t.field_display_volume, { exact: true }).fill('60');
	await page.getByLabel(t.field_system_volume, { exact: true }).fill('72');
	await page.getByRole('button', { name: t.tank_create_submit }).click();
	await page.getByRole('link', { name: t.ls_title }).click();
	await expect(page.getByText(t.ls_empty)).toBeVisible();

	// Add a private species from the form, then two fish of it.
	await page.getByRole('link', { name: t.ls_add }).click();
	await page.getByRole('button', { name: t.ls_species_add_own }).click();
	await page.getByLabel(t.ls_name_vi).fill('Cá hề cam');
	await page.getByLabel(t.ls_name_scientific).fill('Amphiprion percula');
	await page.getByRole('button', { name: t.ls_species_save }).click();
	await expect(page.getByLabel(t.ls_display_name_label)).toHaveValue('Cá hề cam');
	await page.getByLabel(t.ls_quantity_label).fill('2');
	await expectAccessible(page);
	await page.getByRole('button', { name: t.ls_add }).click();
	await expect(page.getByRole('heading', { name: 'Cá hề cam', level: 1 })).toBeVisible();
	const count = page.getByRole('region', { name: t.ls_quantity_label });
	await expect(count.getByText('2', { exact: true })).toBeVisible();

	// One died.
	const record = page.getByRole('region', { name: t.ls_record_title });
	await record.getByLabel(t.ls_event_type_label).selectOption('died');
	await record.getByLabel(t.ls_event_quantity_label).fill('1');
	await record.getByRole('button', { name: t.ls_record }).click();
	await expect(count.getByText('1', { exact: true })).toBeVisible();

	// Moving out more than are present is refused; nothing changes.
	await record.getByLabel(t.ls_event_type_label).selectOption('transferred_out');
	await record.getByLabel(t.ls_event_quantity_label).fill('5');
	await record.getByRole('button', { name: t.ls_record }).click();
	await expect(record.getByRole('alert')).toContainText(t.err_quantity_underflow);
	await expect(count.getByText('1', { exact: true })).toBeVisible();

	// An observation needs a note and no quantity.
	await record.getByLabel(t.ls_event_type_label).selectOption('health_observation');
	await expect(record.getByLabel(t.ls_event_quantity_label)).toHaveCount(0);
	await record.getByLabel(t.wc_notes_label).fill('Bơi bình thường, ăn tốt');
	await record.getByRole('button', { name: t.ls_record }).click();
	const history = page.getByRole('region', { name: t.wc_history_title });
	await expect(history.getByText('Bơi bình thường, ăn tốt')).toBeVisible();
	await expectAccessible(page);

	// Correct the count: a reconciliation event keeps the history.
	await count.getByRole('button', { name: t.ls_reconcile }).click();
	const dialog = page.getByRole('dialog', { name: t.ls_reconcile_title });
	await dialog.getByLabel(t.ls_count_label).fill('3');
	await dialog.getByRole('button', { name: t.ls_reconcile }).click();
	await expect(count.getByText('3', { exact: true })).toBeVisible();
	await expect(history.getByText(t.ls_ev_reconciliation)).toBeVisible();
	await expect(history.getByText(t.ls_ev_died)).toBeVisible();
});
