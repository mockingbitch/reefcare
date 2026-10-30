// Signs up one verified account for the module tests and saves its session, so a run
// registers a handful of accounts at most: registration is limited to 5 per hour per IP
// (docs/api-guidelines.md §6), and every e2e request comes from the same address.
// The registration flow itself is still tested end to end in flows.e2e.ts.
import { chromium, type FullConfig } from '@playwright/test';
import { mkdirSync } from 'node:fs';
import { dirname } from 'node:path';
import { signUp, uniqueEmail } from './helpers';

export const storageState = 'e2e/.auth/keeper.json';

export default async function globalSetup(config: FullConfig) {
	const { baseURL, locale, timezoneId } = config.projects[0].use;
	const browser = await chromium.launch();
	const page = await browser.newPage({ baseURL, locale, timezoneId });
	await signUp(page, uniqueEmail('keeper'), 'Lan');
	mkdirSync(dirname(storageState), { recursive: true });
	await page.context().storageState({ path: storageState });
	await browser.close();
}
