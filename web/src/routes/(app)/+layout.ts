import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import { session } from '#lib/session.svelte.js';
import type { LayoutLoad } from './$types';

// Every page in this group needs a session; others are sent to sign in and come back.
export const load: LayoutLoad = async ({ parent, url }) => {
	await parent();
	if (!session.user) {
		redirect(
			307,
			`${resolve('/(auth)/login')}?next=${encodeURIComponent(url.pathname + url.search)}`
		);
	}
};
