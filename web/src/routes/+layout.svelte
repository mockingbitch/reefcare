<script lang="ts">
	import '../app.css';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import favicon from '#lib/assets/favicon.svg';
	import { api, type ApiError, type User } from '#lib/api/client.js';
	import Alert from '#lib/components/Alert.svelte';
	import { errorMessage } from '#lib/errors.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale, setLocale, locales } from '#lib/paraglide/runtime.js';
	import { unreadBadge } from '#lib/notifications.js';
	import { session } from '#lib/session.svelte.js';
	import { unread } from '#lib/unread.svelte.js';
	import type { LayoutProps } from './$types';

	let { children }: LayoutProps = $props();

	const localeNames: Record<string, () => string> = { vi: m.lang_vi, en: m.lang_en };
	type Locale = (typeof locales)[number];

	$effect(() => {
		document.documentElement.lang = getLocale();
	});

	async function changeLocale(e: Event) {
		const next = (e.currentTarget as HTMLSelectElement).value as Locale;
		// Keep the profile in sync so emails arrive in the same language; otherwise the
		// next page load would switch back to the profile's language.
		if (session.user) {
			let r = await api<User>('PATCH', '/me', { version: session.user.version, locale: next });
			if (!r.ok && r.error.code === 'version_conflict') {
				const me = await session.load(fetch, true);
				if (me) r = await api<User>('PATCH', '/me', { version: me.version, locale: next });
			}
			if (r.ok) session.user = r.data;
		}
		setLocale(next);
	}

	// A failed sign-out must not look successful: on a shared computer the server
	// session would stay valid. 401 means the session is already gone.
	let logoutError = $state<ApiError | null>(null);
	async function logout() {
		logoutError = null;
		const r = await api('POST', '/auth/logout');
		if (!r.ok && r.error.status !== 401) {
			logoutError = r.error;
			return;
		}
		session.clear();
		unread.clear();
		await goto(resolve('/(auth)/login'));
	}

	let resent = $state(false);
	async function resend() {
		const r = await api('POST', '/auth/verify-email/resend');
		resent = r.ok || (!r.ok && r.error.code === 'already_verified');
	}

	const current = (path: string) => (page.url.pathname.startsWith(path) ? 'page' : undefined);

	// Keep the unread count fresh on every navigation while signed in.
	$effect(() => {
		void page.url.pathname;
		if (session.user) unread.refresh();
		else unread.clear();
	});
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
	<title>{m.app_name()}</title>
</svelte:head>

<a class="skip-link" href="#main">{m.skip_to_content()}</a>

<header class="site-header">
	<div class="bar">
		<a class="brand" href={resolve('/')}>{m.app_name()}</a>
		<nav aria-label={m.nav_main()}>
			<ul>
				{#if session.user}
					<li>
						<a href={resolve('/(app)/tanks')} aria-current={current(resolve('/(app)/tanks'))}
							>{m.nav_tanks()}</a
						>
					</li>
					<li>
						<a
							href={resolve('/(app)/notifications')}
							aria-current={current(resolve('/(app)/notifications'))}
							>{m.nav_notifications()}{#if unread.count > 0}
								<span class="badge"
									><span aria-hidden="true">{unreadBadge(unread.count, unread.capped)}</span><span
										class="visually-hidden"
										>{m.nav_unread({ count: unreadBadge(unread.count, unread.capped) })}</span
									></span
								>{/if}</a
						>
					</li>
					<li>
						<a
							href={resolve('/(app)/settings/profile')}
							aria-current={current(resolve('/(app)/settings/profile'))}>{m.nav_profile()}</a
						>
					</li>
					<li>
						<a
							href={resolve('/(app)/settings/security')}
							aria-current={current(resolve('/(app)/settings/security'))}>{m.nav_security()}</a
						>
					</li>
					<li><button type="button" class="link" onclick={logout}>{m.nav_logout()}</button></li>
				{:else}
					<li><a href={resolve('/(auth)/login')}>{m.nav_login()}</a></li>
					<li><a href={resolve('/(auth)/register')}>{m.nav_register()}</a></li>
				{/if}
			</ul>
		</nav>
		<label class="lang">
			<span class="visually-hidden">{m.language()}</span>
			<select value={getLocale()} onchange={changeLocale}>
				{#each locales as l (l)}
					<option value={l}>{localeNames[l]?.() ?? l}</option>
				{/each}
			</select>
		</label>
	</div>
</header>

{#if session.user && !session.user.email_verified}
	<div class="banner" role="status">
		<span aria-hidden="true">ℹ</span>
		{#if resent}
			{m.resend_sent()}
		{:else}
			{m.unverified_banner()}
			<button type="button" class="link" onclick={resend}>{m.resend_verification()}</button>
		{/if}
	</div>
{/if}

<main id="main" tabindex="-1">
	{#if logoutError}<Alert kind="error">{errorMessage(logoutError)}</Alert>{/if}
	{@render children()}
</main>

<style>
	.site-header {
		background: var(--surface);
		border-bottom: 1px solid var(--border);
	}
	.bar {
		max-width: 64rem;
		margin: 0 auto;
		padding: 0.5rem 1rem;
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.5rem 1rem;
	}
	.brand {
		font-weight: 700;
		font-size: 1.15rem;
		text-decoration: none;
		margin-right: auto;
	}
	nav ul {
		display: flex;
		flex-wrap: wrap;
		gap: 0.25rem 1rem;
		list-style: none;
		margin: 0;
		padding: 0;
		align-items: center;
	}
	nav a[aria-current='page'] {
		font-weight: 700;
		text-decoration-thickness: 3px;
	}
	.lang select {
		width: auto;
		min-height: 40px;
		padding: 0.3rem 0.5rem;
	}
	button.link {
		background: none;
		border: none;
		color: var(--accent);
		text-decoration: underline;
		padding: 0;
		min-height: 44px;
		font-weight: inherit;
	}
	.banner {
		background: var(--info-bg);
		color: var(--info-fg);
		padding: 0.5rem 1rem;
		text-align: center;
	}
	main:focus {
		outline: none;
	}
</style>
