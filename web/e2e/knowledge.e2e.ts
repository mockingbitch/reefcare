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

// Publishing needs two editors (four eyes) granted on the command line; the whole
// workflow is covered by internal/knowledge's integration tests. Here: the reader pages
// and the editor pages refusing a plain keeper.
test('knowledge base for readers, editor pages refused to keepers', async ({ page }) => {
	await page.goto('/tanks');
	await page.getByRole('link', { name: t.nav_knowledge }).click();
	await expect(page.getByRole('heading', { name: t.kb_title, level: 1 })).toBeVisible();
	await expect(page.getByText(t.kb_empty)).toBeVisible();
	await expect(page.getByRole('link', { name: t.nav_knowledge_admin })).toHaveCount(0);
	await expectAccessible(page);

	await page.goto('/knowledge/not-published');
	await expect(page.getByRole('alert')).toBeVisible();

	await page.goto('/admin/knowledge');
	await expect(page.getByText(t.kb_admin_forbidden)).toBeVisible();
	await expectAccessible(page);
});
