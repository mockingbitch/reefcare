import AxeBuilder from '@axe-core/playwright';
import { expect, test, type Page } from '@playwright/test';
import { password, signUp, t, uniqueEmail } from './helpers';

async function expectAccessible(page: Page) {
	const results = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag22aa'])
		.analyze();
	expect(results.violations.map((v) => `${v.id}: ${v.help}`)).toEqual([]);
}

test('signed-out visitors are sent to sign in with a return path', async ({ page }) => {
	await page.goto('/tanks/new');
	await expect(page).toHaveURL(/\/login\?next=%2Ftanks%2Fnew$/);
	await expect(page.getByRole('heading', { name: t.login_title })).toBeVisible();
	await expectAccessible(page);
});

test('the interface switches language', async ({ page }) => {
	await page.goto('/login');
	await page.getByLabel(t.language).selectOption('en');
	await expect(page.getByRole('heading', { name: 'Sign in' })).toBeVisible();
	await page.getByLabel('Language').selectOption('vi');
	await expect(page.getByRole('heading', { name: t.login_title })).toBeVisible();
});

test('wrong credentials show a localized error', async ({ page }) => {
	await page.goto('/login');
	await page.getByLabel(t.field_email).fill(uniqueEmail('nobody'));
	await page.getByLabel(t.field_password).fill('not the password at all');
	await page.getByRole('button', { name: t.login_submit }).click();
	await expect(page.getByRole('alert')).toHaveText(new RegExp(t.err_invalid_credentials));
});

test('register, verify, create a tank and share it read-only', async ({ page, browser }) => {
	test.setTimeout(90_000);
	const owner = uniqueEmail('lan');
	const viewerEmail = uniqueEmail('minh');
	await signUp(page, owner, 'Lan');
	await expectAccessible(page);

	// Empty state, then create a tank.
	await expect(page.getByText(t.tanks_empty)).toBeVisible();
	await page.getByRole('link', { name: t.tanks_empty_cta }).click();
	await page.getByLabel(t.field_name).fill('Nano SPS 60');
	await page.getByLabel(t.field_tank_type).selectOption('sps_dominant');
	await page.getByLabel(t.field_display_volume).fill('60');
	await page.getByLabel(t.field_system_volume).fill('50');
	await expect(page.getByText(t.warn_system_below_display)).toBeVisible();
	await page.getByLabel(t.field_system_volume).fill('72,5');
	await expect(page.getByText(t.warn_system_below_display)).toBeHidden();
	await page.getByLabel(t.field_length).fill('60');
	await page.getByRole('button', { name: t.tank_create_submit }).click();
	// Dimensions must be all-or-nothing; nothing was sent yet.
	await expect(page.getByText(t.f_dimensions_all_or_none)).toBeVisible();
	await page.getByLabel(t.field_width).fill('40');
	await page.getByLabel(t.field_height).fill('35');
	await page.getByRole('button', { name: t.tank_create_submit }).click();

	await expect(page.getByRole('heading', { name: 'Nano SPS 60', level: 1 })).toBeVisible();
	await expect(page.getByText('72,5 L (ước tính)')).toBeVisible();
	await expectAccessible(page);
	const tankURL = page.url();

	// Server-side validation errors land on the right field.
	await page.getByRole('link', { name: t.tank_edit }).click();
	await page.getByLabel(t.field_display_volume).fill('-5');
	await page.getByRole('button', { name: t.tank_edit_submit }).click();
	await expect(page.getByText(t.f_must_be_positive)).toBeVisible();
	await page.getByLabel(t.field_display_volume).fill('65');
	await page.getByRole('button', { name: t.tank_edit_submit }).click();
	await expect(page.getByText('65 L', { exact: true })).toBeVisible();

	// A second account to share with.
	const viewerContext = await browser.newContext({ locale: 'vi-VN' });
	const viewer = await viewerContext.newPage();
	await signUp(viewer, viewerEmail, 'Minh');

	await page.getByRole('link', { name: t.tank_sharing }).click();
	await page.getByRole('textbox', { name: t.field_email }).fill(uniqueEmail('ghost'));
	await page.getByRole('button', { name: t.share_submit }).click();
	await expect(page.getByText(t.f_grantee_not_found)).toBeVisible();
	await page.getByRole('textbox', { name: t.field_email }).fill(viewerEmail);
	await page.getByRole('button', { name: t.share_submit }).click();
	await expect(page.getByText(t.share_added.replace('{name}', 'Minh'))).toBeVisible();
	await expectAccessible(page);

	// The viewer sees the tank, read-only.
	await viewer.goto(tankURL);
	await expect(viewer.getByRole('heading', { name: 'Nano SPS 60', level: 1 })).toBeVisible();
	await expect(viewer.getByText(t.viewer_notice)).toBeVisible();
	await expect(viewer.getByRole('link', { name: t.tank_edit })).toHaveCount(0);

	// Revocation takes effect at once.
	await page.getByRole('button', { name: t.share_revoke_label.replace('{name}', 'Minh') }).click();
	await expect(page.getByText(t.share_revoked)).toBeVisible();
	await viewer.reload();
	await expect(viewer.getByRole('heading', { name: t.not_found_title })).toBeVisible();
	await viewerContext.close();

	// Archive behind a confirmation dialog.
	await page.goto(tankURL);
	await page.getByRole('button', { name: t.tank_archive }).click();
	const dialog = page.getByRole('dialog', { name: t.archive_confirm_title });
	await expect(dialog).toBeVisible();
	await dialog.getByRole('button', { name: t.archive_confirm }).click();
	await expect(page.getByText(t.archived_notice)).toBeVisible();
	await expect(page.getByRole('link', { name: t.tank_edit })).toHaveCount(0);

	// Signing out and back in keeps the data.
	await page.getByRole('button', { name: t.nav_logout }).click();
	await expect(page.getByRole('heading', { name: t.login_title })).toBeVisible();

	// A crafted ?next= that the browser would turn into //evil.example stays on this site.
	await page.goto('/login?next=%2F%09%2Fevil.example%2Flogin');
	await page.getByLabel(t.field_email).fill(owner);
	await page.getByLabel(t.field_password).fill(password);
	await page.getByRole('button', { name: t.login_submit }).click();
	await expect(page.getByRole('heading', { name: t.tanks_title, level: 1 })).toBeVisible();
	expect(new URL(page.url()).host).toBe(new URL(test.info().project.use.baseURL ?? '').host);
});
