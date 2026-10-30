import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import { session } from '#lib/session.svelte.js';
import type { LayoutLoad } from './$types';

// Sign-in and sign-up make no sense with a session; email-link pages work either way.
export const load: LayoutLoad = async ({ parent, url }) => {
	await parent();
	if (
		session.user &&
		(url.pathname === resolve('/(auth)/login') || url.pathname === resolve('/(auth)/register'))
	) {
		redirect(307, resolve('/(app)/tanks'));
	}
};
