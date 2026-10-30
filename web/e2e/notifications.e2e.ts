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

// Reminders are created by a 15-minute background scan, so this covers the page, the
// navigation entry and the preferences; rendering of each kind is unit-tested
// (src/lib/notifications.spec.ts) and the scan in internal/notifications.
test('notification centre and in-app reminder preferences', async ({ page }) => {
	await page.goto('/tanks');
	await page.getByRole('link', { name: new RegExp(`^${t.nav_notifications}`) }).click();
	await expect(page.getByRole('heading', { name: t.nt_title, level: 1 })).toBeVisible();
	await expect(page.getByRole('button', { name: t.nt_read_all })).toBeDisabled();
	await page.getByLabel(t.nt_only_unread).check();
	await expect(page.getByText(t.nt_empty_unread)).toBeVisible();
	await expectAccessible(page);

	// In-app reminders are on by default and can be turned off per category.
	await page.getByRole('link', { name: t.nt_settings }).click();
	const dosing = page.getByLabel(t.notify_inapp_dosing);
	await expect(dosing).toBeChecked();
	await dosing.uncheck();
	await page.getByRole('button', { name: t.save }).click();
	await expect(page.getByText(t.profile_saved)).toBeVisible();
	await page.reload();
	await expect(page.getByLabel(t.notify_inapp_dosing)).not.toBeChecked();
	await expect(page.getByLabel(t.notify_inapp_maintenance)).toBeChecked();
});
