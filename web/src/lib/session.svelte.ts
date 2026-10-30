// The signed-in user, shared by every page. The API is the source of truth: the session
// cookie is HttpOnly, so "signed in" means GET /me answered 200.
import { api, onUnauthenticated, type User } from '#lib/api/client.js';
import { getLocale, setLocale, locales } from '#lib/paraglide/runtime.js';

type Locale = (typeof locales)[number];

class Session {
	user = $state<User | null>(null);
	loaded = $state(false);

	/** Loads the current user once per page load (or again with force). */
	async load(fetchFn: typeof fetch = fetch, force = false): Promise<User | null> {
		if (this.loaded && !force) return this.user;
		const r = await api<User>('GET', '/me', undefined, fetchFn);
		this.set(r.ok ? r.data : null);
		this.loaded = true;
		return this.user;
	}

	/** Stores the user and follows the language chosen in their profile. */
	set(user: User | null) {
		this.user = user;
		if (
			user &&
			user.locale !== getLocale() &&
			(locales as readonly string[]).includes(user.locale)
		) {
			setLocale(user.locale as Locale); // reloads the page in the new language
		}
	}

	/**
	 * After sign-in: switches to the profile language if needed (a full page load, so
	 * every message re-renders) and navigates to target.
	 */
	async signIn(user: User, target: string, navigate: (url: string) => Promise<void>) {
		if (user.locale !== getLocale() && (locales as readonly string[]).includes(user.locale)) {
			setLocale(user.locale as Locale, { reload: false });
			location.assign(target);
			return;
		}
		this.user = user;
		this.loaded = true;
		await navigate(target);
	}

	clear() {
		this.user = null;
	}
}

export const session = new Session();

onUnauthenticated(() => session.clear());
