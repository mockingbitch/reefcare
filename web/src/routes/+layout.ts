import { session } from '#lib/session.svelte.js';
import type { LayoutLoad } from './$types';

// Single-page app: everything renders in the browser (docs/frontend.md §2).
export const ssr = false;
export const prerender = false;

export const load: LayoutLoad = async ({ fetch }) => {
	await session.load(fetch);
	return {};
};
