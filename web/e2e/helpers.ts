import { expect, type APIRequestContext, type Page } from '@playwright/test';
import vi from '../messages/vi.json' with { type: 'json' };

export const t = vi;
export const password = 'correct horse battery staple';

const mailpitURL = process.env.MAILPIT_URL ?? 'http://localhost:4003';

/** A unique address per run so tests can share the dev database. */
export function uniqueEmail(name: string): string {
	return `${name}.${Date.now()}.${Math.random().toString(36).slice(2, 8)}@e2e.reefcare.test`;
}

/** Waits for the newest email to `to` and returns its #token= link. */
export async function emailLink(request: APIRequestContext, to: string): Promise<string> {
	let link = '';
	await expect
		.poll(
			async () => {
				const res = await request.get(`${mailpitURL}/api/v1/search`, {
					params: { query: `to:"${to}"` }
				});
				const list = (await res.json()) as { messages?: { ID: string }[] };
				const id = list.messages?.[0]?.ID;
				if (!id) return '';
				const msg = (await (await request.get(`${mailpitURL}/api/v1/message/${id}`)).json()) as {
					Text: string;
				};
				link = msg.Text.match(/https?:\/\/\S+#token=[A-Za-z0-9_-]+/)?.[0] ?? '';
				return link;
			},
			{ timeout: 10_000 }
		)
		.not.toBe('');
	return link;
}

/** Registers, verifies and signs in through the UI. */
export async function signUp(page: Page, email: string, name: string) {
	await page.goto('/register');
	await page.getByLabel(t.field_display_name).fill(name);
	await page.getByLabel(t.field_email).fill(email);
	await page.getByLabel(t.field_password, { exact: true }).fill(password);
	await page.getByRole('button', { name: t.register_submit }).click();
	await expect(page.getByRole('heading', { name: t.register_sent_title })).toBeVisible();

	await page.goto(await emailLink(page.request, email));
	await expect(page.getByText(t.verify_success)).toBeVisible();
	await expect(page).not.toHaveURL(/token=/); // the token is removed from the address bar

	await signIn(page, email);
}

export async function signIn(page: Page, email: string) {
	await page.goto('/login');
	await page.getByLabel(t.field_email).fill(email);
	await page.getByLabel(t.field_password).fill(password);
	await page.getByRole('button', { name: t.login_submit }).click();
	await expect(page.getByRole('heading', { name: t.tanks_title, level: 1 })).toBeVisible();
}
