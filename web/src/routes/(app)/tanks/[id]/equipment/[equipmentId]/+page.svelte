<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import {
		api,
		type ApiError,
		type Equipment,
		type EquipmentConfiguration,
		type MaintenanceEvent,
		type MaintenancePlan,
		type Page,
		type Tank
	} from '#lib/api/client.js';
	import Alert from '#lib/components/Alert.svelte';
	import ConfirmDialog from '#lib/components/ConfirmDialog.svelte';
	import Field from '#lib/components/Field.svelte';
	import SettingsEditor from '#lib/components/SettingsEditor.svelte';
	import {
		categoryLabel,
		diffSettings,
		formatSetting,
		rowsToSettings,
		settingsToRows,
		warrantyExpired,
		type Settings,
		type SettingRow
	} from '#lib/equipment.js';
	import { errorMessage, fieldErrors } from '#lib/errors.js';
	import { formatDate, formatDateTime, today } from '#lib/format.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import { session } from '#lib/session.svelte.js';
	import { fromLocalInput, toLocalInput } from '#lib/water-changes.js';

	const id = $derived(page.params.id ?? '');
	const equipmentId = $derived(page.params.equipmentId ?? '');
	const locale = getLocale();
	const tz = $derived(session.user?.timezone ?? 'Asia/Ho_Chi_Minh');

	let tank = $state<Tank | null>(null);
	let device = $state<Equipment | null>(null);
	let versions = $state<EquipmentConfiguration[]>([]);
	let plans = $state<MaintenancePlan[]>([]);
	let events = $state<MaintenanceEvent[]>([]);
	let loading = $state(true);
	let loadError = $state<ApiError | null>(null);

	const canEdit = $derived(
		tank?.access === 'owner' && tank.status !== 'archived' && !!device && !device.removed_at
	);

	async function load() {
		loading = true;
		loadError = null;
		const eid = encodeURIComponent(equipmentId);
		const tid = encodeURIComponent(id);
		const [t, d, v, p, ev] = await Promise.all([
			api<Tank>('GET', `/tanks/${tid}`),
			api<Equipment>('GET', `/equipment/${eid}`),
			api<Page<EquipmentConfiguration>>('GET', `/equipment/${eid}/configurations`),
			api<Page<MaintenancePlan>>('GET', `/tanks/${tid}/maintenance-plans`),
			api<Page<MaintenanceEvent>>(
				'GET',
				`/tanks/${tid}/maintenance-events?equipment_id=${eid}&limit=20`
			)
		]);
		loading = false;
		for (const r of [t, d, v, p, ev]) if (!r.ok) return void (loadError = r.error);
		if (!t.ok || !d.ok || !v.ok || !p.ok || !ev.ok) return;
		tank = t.data;
		device = d.data;
		versions = v.data.items;
		plans = p.data.items.filter((x) => x.equipment_id === d.data.id);
		events = ev.data.items;
	}

	$effect(() => {
		void equipmentId;
		load();
	});

	// --- New configuration version -----------------------------------------------------------
	let editing = $state(false);
	let rows = $state<SettingRow[]>([]);
	let effectiveAt = $state('');
	let note = $state('');
	let busy = $state(false);
	let configError = $state<ApiError | null>(null);
	const cfe = $derived(fieldErrors(configError));

	function startConfig() {
		rows = settingsToRows(device?.configuration?.settings as Settings | undefined);
		if (rows.length === 0) rows = [{ key: '', kind: 'text', value: '' }];
		effectiveAt = toLocalInput(new Date().toISOString(), tz);
		note = '';
		configError = null;
		editing = true;
	}

	async function saveConfig() {
		if (!device) return;
		busy = true;
		configError = null;
		const r = await api<EquipmentConfiguration>('POST', `/equipment/${device.id}/configurations`, {
			settings: rowsToSettings(rows),
			effective_at: fromLocalInput(effectiveAt, tz) || undefined,
			note: note.trim() || null
		});
		busy = false;
		if (!r.ok) return void (configError = r.error);
		editing = false;
		await load();
	}

	// --- Remove ---------------------------------------------------------------------------------
	let removeOpen = $state(false);
	let removedAt = $state('');
	let reason = $state('');
	let removeError = $state<ApiError | null>(null);
	const rfe = $derived(fieldErrors(removeError));

	function openRemove() {
		removedAt = toLocalInput(new Date().toISOString(), tz);
		reason = '';
		removeError = null;
		removeOpen = true;
	}

	async function remove() {
		if (!device) return;
		busy = true;
		const r = await api<Equipment>('POST', `/equipment/${device.id}/remove`, {
			removed_at: fromLocalInput(removedAt, tz) || undefined,
			reason: reason.trim() || null
		});
		busy = false;
		if (!r.ok) return void (removeError = r.error);
		removeOpen = false;
		await load();
	}

	function changeSummary(i: number): string {
		const v = versions[i];
		const prev = versions[i + 1]?.settings as Settings | undefined;
		const d = diffSettings(prev, v.settings as Settings);
		const parts = [
			d.changed.length ? m.eq_diff_changed({ keys: d.changed.join(', ') }) : '',
			d.added.length ? m.eq_diff_added({ keys: d.added.join(', ') }) : '',
			d.removed.length ? m.eq_diff_removed({ keys: d.removed.join(', ') }) : ''
		].filter(Boolean);
		return parts.length ? parts.join(' · ') : m.eq_diff_none();
	}
</script>

<svelte:head><title>{device?.name ?? m.eq_title()} · {m.app_name()}</title></svelte:head>

<p><a href={resolve('/(app)/tanks/[id]/equipment', { id })}>← {m.eq_title()}</a></p>

{#if loading && !device}
	<p role="status">{m.loading()}</p>
{:else if loadError}
	{#if loadError.code === 'not_found'}
		<h1>{m.not_found_title()}</h1>
		<p>{m.not_found_body()}</p>
	{:else}
		<Alert kind="error">
			{errorMessage(loadError)}
			<button type="button" class="secondary" onclick={load}>{m.retry()}</button>
		</Alert>
	{/if}
{:else if device && tank}
	<h1>{device.name}</h1>
	<p class="meta">
		<span class="badge">{categoryLabel(device.category)}</span>
		{#if device.removed_at}<span class="badge">{m.eq_removed_badge()}</span>{/if}
	</p>
	{#if device.removed_at}
		<Alert kind="info">
			{m.eq_removed_on({ when: formatDateTime(device.removed_at, locale, tz) })}
			{#if device.removal_reason}— {device.removal_reason}{/if}
		</Alert>
	{/if}

	<section class="card" aria-labelledby="info">
		<h2 id="info">{m.tank_info()}</h2>
		<dl class="facts">
			<dt>{m.eq_manufacturer_label()}</dt>
			<dd>{device.manufacturer ?? m.not_set()}</dd>
			<dt>{m.eq_model_label()}</dt>
			<dd>{device.model ?? m.not_set()}</dd>
			<dt>{m.eq_serial_label()}</dt>
			<dd>{device.serial_number ?? m.not_set()}</dd>
			<dt>{m.eq_purchased_label()}</dt>
			<dd>{device.purchased_on ? formatDate(device.purchased_on, locale) : m.not_set()}</dd>
			<dt>{m.eq_warranty_label()}</dt>
			<dd>
				{#if !device.warranty_until}
					{m.not_set()}
				{:else if warrantyExpired(device.warranty_until, today(tz))}
					<span aria-hidden="true">⚠</span>
					{m.eq_warranty_expired({ date: formatDate(device.warranty_until, locale) })}
				{:else}
					{formatDate(device.warranty_until, locale)}
				{/if}
			</dd>
			<dt>{m.eq_installed_label()}</dt>
			<dd>{formatDateTime(device.installed_at, locale, tz)}</dd>
		</dl>
		{#if device.notes}<p class="notes">{device.notes}</p>{/if}
		{#if canEdit}
			<div class="actions">
				<a
					class="button secondary"
					href={resolve('/(app)/tanks/[id]/equipment/[equipmentId]/edit', { id, equipmentId })}
					>{m.wc_edit()}</a
				>
				<button type="button" class="secondary" onclick={openRemove}>{m.eq_remove()}</button>
			</div>
		{/if}
	</section>

	<section class="card" aria-labelledby="config">
		<h2 id="config">{m.eq_config_title()}</h2>
		{#if device.configuration}
			<p class="muted">
				{m.eq_config_version({ n: String(device.configuration.version_no) })} · {m.eq_effective({
					when: formatDateTime(device.configuration.effective_at, locale, tz)
				})}
			</p>
			<dl class="facts">
				{#each Object.entries(device.configuration.settings) as [k, v] (k)}
					<dt>{k}</dt>
					<dd>{formatSetting(v, locale)}</dd>
				{/each}
			</dl>
		{:else}
			<p class="muted">{m.eq_config_empty()}</p>
		{/if}

		{#if canEdit && !editing}
			<button type="button" onclick={startConfig}>{m.eq_config_new()}</button>
		{/if}
		{#if editing}
			<form
				novalidate
				onsubmit={(e) => {
					e.preventDefault();
					saveConfig();
				}}
			>
				{#if configError && configError.code !== 'validation_failed'}
					<Alert kind="error">{errorMessage(configError)}</Alert>
				{/if}
				<SettingsEditor bind:rows errors={cfe} />
				<Field
					label={m.eq_effective_label()}
					name="effective_at"
					type="datetime-local"
					bind:value={effectiveAt}
					hint={m.eq_effective_hint()}
					error={cfe.effective_at}
				/>
				<Field label={m.eq_config_note()} name="note" optional bind:value={note} error={cfe.note} />
				<div class="actions">
					<button type="submit" disabled={busy}>{busy ? m.saving() : m.eq_config_save()}</button>
					<button type="button" class="secondary" onclick={() => (editing = false)}
						>{m.cancel()}</button
					>
				</div>
			</form>
		{/if}

		{#if versions.length > 1}
			<details>
				<summary>{m.eq_config_history()} ({versions.length})</summary>
				<ol class="versions">
					{#each versions as v, i (v.id)}
						<li>
							<strong>{m.eq_config_version({ n: String(v.version_no) })}</strong>
							· <time datetime={v.effective_at}>{formatDateTime(v.effective_at, locale, tz)}</time>
							<p>{changeSummary(i)}</p>
							{#if v.note}<p class="muted">{v.note}</p>{/if}
						</li>
					{/each}
				</ol>
			</details>
		{/if}
	</section>

	<section class="card" aria-labelledby="maint">
		<h2 id="maint">{m.tank_section_maintenance()}</h2>
		{#if plans.length === 0 && events.length === 0}
			<p class="muted">{m.mt_device_empty()}</p>
		{/if}
		{#if plans.length > 0}
			<ul>
				{#each plans as p (p.id)}
					<li>
						{p.title} · {m.mt_every_days({ n: String(p.interval_days) })} · {m.mt_next_due({
							date: formatDate(p.next_due_on, locale)
						})}
						{#if p.overdue}<span class="badge">⚠ {m.wc_status_overdue()}</span>{/if}
					</li>
				{/each}
			</ul>
		{/if}
		{#if events.length > 0}
			<h3>{m.wc_history_title()}</h3>
			<ul>
				{#each events as ev (ev.id)}
					<li>
						<time datetime={ev.performed_at}>{formatDateTime(ev.performed_at, locale, tz)}</time>
						· {ev.title}
					</li>
				{/each}
			</ul>
		{/if}
		<p class="links">
			{#if canEdit}
				<a
					href="{resolve('/(app)/tanks/[id]/maintenance/plans/new', {
						id
					})}?equipment_id={encodeURIComponent(equipmentId)}">{m.mt_plan_new()}</a
				>
			{/if}
			<a href={resolve('/(app)/tanks/[id]/maintenance', { id })}>{m.mt_open_page()}</a>
		</p>
	</section>

	<ConfirmDialog
		bind:open={removeOpen}
		title={m.eq_remove_title()}
		confirmLabel={m.eq_remove()}
		danger
		{busy}
		onconfirm={remove}
	>
		<p>{m.eq_remove_body()}</p>
		{#if removeError && removeError.code !== 'validation_failed'}
			<Alert kind="error">{errorMessage(removeError)}</Alert>
		{/if}
		<Field
			label={m.eq_removed_at_label()}
			name="removed_at"
			type="datetime-local"
			bind:value={removedAt}
			hint={m.rec_timezone_note({ tz })}
			error={rfe.removed_at}
		/>
		<Field
			label={m.wc_skip_reason_label()}
			name="reason"
			optional
			bind:value={reason}
			error={rfe.reason}
		/>
	</ConfirmDialog>
{/if}

<style>
	.meta {
		display: flex;
		flex-wrap: wrap;
		gap: 0.4rem;
	}
	.notes {
		white-space: pre-line;
	}
	.versions {
		padding-left: 1.25rem;
	}
	.versions p {
		margin: 0.15rem 0 0.5rem;
	}
	h3 {
		font-size: 1rem;
		margin: 1rem 0 0.25rem;
	}
</style>
