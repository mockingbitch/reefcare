<script lang="ts">
	import { api, type ApiError, type User } from '#lib/api/client.js';
	import Alert from '#lib/components/Alert.svelte';
	import Field from '#lib/components/Field.svelte';
	import SelectField from '#lib/components/SelectField.svelte';
	import { errorMessage, fieldErrors } from '#lib/errors.js';
	import { experienceLabel } from '#lib/labels.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale, setLocale, locales } from '#lib/paraglide/runtime.js';
	import { session } from '#lib/session.svelte.js';

	type Locale = (typeof locales)[number];

	// Display-unit choices accepted by the API (UnitPrefs in api/openapi.yaml).
	const unitChoices: Record<string, string[]> = {
		volume: ['L', 'gal_us'],
		length: ['cm', 'in'],
		temperature: ['C', 'F'],
		alkalinity: ['dKH', 'meq/L', 'ppm'],
		salinity: ['SG', 'ppt']
	};
	const unitLabels: Record<string, () => string> = {
		volume: m.unit_volume,
		length: m.unit_length,
		temperature: m.unit_temperature,
		alkalinity: m.unit_alkalinity,
		salinity: m.unit_salinity
	};
	// In-app reminders per category default to on; email channels are opt-in (API NotificationPrefs).
	const reminderKeys = [
		'reminders_dosing',
		'reminders_water_change',
		'reminders_maintenance'
	] as const;
	const notifyKeys = ['email_reminders', 'email_assessments', 'email_digest'] as const;
	const notifyLabels: Record<string, () => string> = {
		reminders_dosing: m.notify_inapp_dosing,
		reminders_water_change: m.notify_inapp_water_change,
		reminders_maintenance: m.notify_inapp_maintenance,
		email_reminders: m.notify_reminders,
		email_assessments: m.notify_assessments,
		email_digest: m.notify_digest
	};

	const u = session.user as User;
	let displayName = $state(u.display_name);
	let username = $state(u.username ?? '');
	let bio = $state(u.bio ?? '');
	let experience = $state(u.experience_level ?? '');
	let locale = $state(u.locale);
	let timezone = $state(u.timezone);
	// Every key starts as '' (the "default" option): binding undefined into SelectField, whose
	// value has a fallback, throws (svelte props_invalid_value) for accounts without unit prefs.
	let units = $state<Record<string, string>>(
		Object.fromEntries(
			Object.keys(unitChoices).map((k) => [k, (u.unit_prefs as Record<string, string>)[k] ?? ''])
		)
	);
	let notify = $state<Record<string, boolean>>({
		...Object.fromEntries(reminderKeys.map((k) => [k, true])),
		...(u.notification_prefs as Record<string, boolean>)
	});
	let busy = $state(false);
	let error = $state<ApiError | null>(null);
	let saved = $state(false);
	const fields = $derived(fieldErrors(error));

	const zones = (() => {
		try {
			return Intl.supportedValuesOf('timeZone');
		} catch {
			return [u.timezone];
		}
	})();
	const zoneOptions = $derived(
		(zones.includes(timezone) ? zones : [timezone, ...zones]).map((z) => ({ value: z, label: z }))
	);
	const expOptions = [
		{ value: '', label: m.exp_none() },
		...['beginner', 'intermediate', 'advanced', 'professional'].map((e) => ({
			value: e,
			label: experienceLabel(e)
		}))
	];
	const localeOptions = [
		{ value: 'vi', label: m.lang_vi() },
		{ value: 'en', label: m.lang_en() }
	];

	async function save(e: SubmitEvent) {
		e.preventDefault();
		if (!session.user) return;
		busy = true;
		error = null;
		saved = false;
		const unitPatch: Record<string, string | null> = {};
		for (const k of Object.keys(unitChoices)) unitPatch[k] = units[k] || null;
		const notifyPatch: Record<string, boolean> = {};
		for (const k of [...reminderKeys, ...notifyKeys]) notifyPatch[k] = !!notify[k];
		const r = await api<User>('PATCH', '/me', {
			version: session.user.version,
			display_name: displayName,
			username: username.trim() === '' ? null : username,
			bio: bio.trim() === '' ? null : bio,
			experience_level: experience === '' ? null : experience,
			locale,
			timezone,
			unit_prefs: unitPatch,
			notification_prefs: notifyPatch
		});
		busy = false;
		if (!r.ok) {
			error = r.error;
			return;
		}
		session.user = r.data;
		saved = true;
		if (r.data.locale !== getLocale()) setLocale(r.data.locale as Locale);
	}
</script>

<svelte:head><title>{m.profile_title()} · {m.app_name()}</title></svelte:head>

<h1>{m.profile_title()}</h1>

<form class="card" onsubmit={save} novalidate>
	{#if error}
		<Alert kind="error">
			{errorMessage(error)}
			{#if error.code === 'version_conflict'}
				<button type="button" class="secondary" onclick={() => location.reload()}
					>{m.retry()}</button
				>
			{/if}
		</Alert>
	{/if}
	{#if saved}<Alert kind="success">{m.profile_saved()}</Alert>{/if}

	<Field
		label={m.field_display_name()}
		name="display_name"
		maxlength={60}
		required
		bind:value={displayName}
		error={fields.display_name}
	/>
	<Field
		label={m.field_username()}
		name="username"
		optional
		autocomplete="username"
		hint={m.username_hint()}
		bind:value={username}
		error={fields.username}
	/>
	<Field label={m.field_bio()} name="bio" multiline optional bind:value={bio} error={fields.bio} />
	<SelectField
		label={m.field_experience()}
		name="experience_level"
		options={expOptions}
		bind:value={experience}
		error={fields.experience_level}
	/>
	<SelectField
		label={m.field_locale()}
		name="locale"
		options={localeOptions}
		bind:value={locale}
		error={fields.locale}
	/>
	<SelectField
		label={m.field_timezone()}
		name="timezone"
		options={zoneOptions}
		hint={m.timezone_hint()}
		bind:value={timezone}
		error={fields.timezone}
	/>

	<fieldset>
		<legend>{m.units_title()}</legend>
		{#each Object.entries(unitChoices) as [key, choices] (key)}
			<SelectField
				label={unitLabels[key]()}
				name={`unit_${key}`}
				options={[
					{ value: '', label: m.unit_default() },
					...choices.map((c) => ({ value: c, label: c }))
				]}
				bind:value={units[key]}
				error={fields[`unit_prefs.${key}`]}
			/>
		{/each}
	</fieldset>

	<fieldset>
		<legend>{m.notifications_title()}</legend>
		<p class="hint">{m.notify_inapp_title()}</p>
		{#each reminderKeys as key (key)}
			<label class="check"
				><input type="checkbox" bind:checked={notify[key]} /> {notifyLabels[key]()}</label
			>
		{/each}
		<p class="hint">{m.notify_email_title()}</p>
		{#each notifyKeys as key (key)}
			<label class="check"
				><input type="checkbox" bind:checked={notify[key]} /> {notifyLabels[key]()}</label
			>
		{/each}
	</fieldset>

	<div class="actions">
		<button type="submit" disabled={busy}>{busy ? m.saving() : m.save()}</button>
	</div>
</form>

<style>
	.check {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		min-height: 44px;
	}
</style>
