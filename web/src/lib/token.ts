import { replaceState } from '$app/navigation';

// Email links carry one-time tokens in the URL fragment (#token=…), which browsers never
// send to servers or in Referer headers (docs/security.md §2).

export function parseFragmentToken(hash: string): string | null {
	const params = new URLSearchParams(hash.replace(/^#/, ''));
	const token = params.get('token');
	return token && /^[A-Za-z0-9_-]{16,128}$/.test(token) ? token : null;
}

/**
 * Reads the token and immediately removes it from the address bar and history, through
 * SvelteKit's router so its own URL state drops the fragment too. Call after mount.
 */
export function takeFragmentToken(): string | null {
	const token = parseFragmentToken(location.hash);
	// Same page without the fragment, not a navigation target, so resolve() doesn't apply.
	// eslint-disable-next-line svelte/no-navigation-without-resolve
	if (location.hash) replaceState(location.pathname + location.search, {});
	return token;
}

/**
 * Returns a post-login redirect only if it stays on this origin. The value is parsed the
 * way the browser will parse it (which strips tabs and newlines, so "/%09/evil.example"
 * would otherwise become "//evil.example"), and control characters or backslashes are
 * rejected outright.
 */
export function safeNext(next: string | null, origin: string = location.origin): string | null {
	// eslint-disable-next-line no-control-regex
	if (!next || !next.startsWith('/') || /[\u0000-\u001f\u007f\\]/.test(next)) return null;
	let u: URL;
	try {
		u = new URL(next, origin);
	} catch {
		return null;
	}
	if (u.origin !== origin) return null;
	return u.pathname + u.search + u.hash;
}
