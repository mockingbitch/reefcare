<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { api, type ApiError, type Tank } from '#lib/api/client.js';
	import Alert from '#lib/components/Alert.svelte';
	import ConfirmDialog from '#lib/components/ConfirmDialog.svelte';
	import TankDashboard from '#lib/components/TankDashboard.svelte';
	import { errorMessage } from '#lib/errors.js';
	import { formatDate, formatDateTime, formatDecimal } from '#lib/format.js';
	import { statusLabel, tankTypeLabel } from '#lib/labels.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import { session } from '#lib/session.svelte.js';

	const id = $derived(page.params.id ?? '');
	let tank = $state<Tank | null>(null);
	let loading = $state(true);
	let error = $state<ApiError | null>(null);
	let actionError = $state<ApiError | null>(null);
	let confirmArchive = $state(false);
	let confirmLeave = $state(false);
	let busy = $state(false);

	const locale = getLocale();
	const tz = $derived(session.user?.timezone);
	const dec = (v: string) => formatDecimal(v, locale);

	async function load() {
		loading = true;
		error = null;
		const r = await api<Tank>('GET', `/tanks/${encodeURIComponent(id)}`);
		loading = false;
		if (r.ok) tank = r.data;
		else error = r.error;
	}

	$effect(() => {
		void id;
		load();
	});

	async function archive() {
		busy = true;
		const r = await api<Tank>('POST', `/tanks/${encodeURIComponent(id)}/archive`);
		busy = false;
		confirmArchive = false;
		if (r.ok) tank = r.data;
		else actionError = r.error;
	}

	async function leave() {
		if (!session.user) return;
		busy = true;
		const r = await api('DELETE', `/tanks/${encodeURIComponent(id)}/shares/${session.user.id}`);
		busy = false;
		confirmLeave = false;
		if (r.ok) await goto(resolve('/(app)/tanks'));
		else actionError = r.error;
	}
</script>

<svelte:head><title>{tank?.name ?? m.tanks_title()} · {m.app_name()}</title></svelte:head>

<p><a href={resolve('/(app)/tanks')}>← {m.nav_tanks()}</a></p>

{#if loading && !tank}
	<p role="status">{m.loading()}</p>
{:else if error}
	{#if error.code === 'not_found'}
		<h1>{m.not_found_title()}</h1>
		<p>{m.not_found_body()}</p>
	{:else}
		<Alert kind="error">
			{errorMessage(error)}
			<button type="button" class="secondary" onclick={load}>{m.retry()}</button>
		</Alert>
	{/if}
{:else if tank}
	<h1>{tank.name}</h1>
	<p class="meta">
		<span class="badge">{tankTypeLabel(tank.tank_type)}</span>
		<span class="badge">{statusLabel(tank.status)}</span>
		<span class="badge">{tank.access === 'owner' ? m.access_owner() : m.access_viewer()}</span>
	</p>

	{#if tank.status === 'archived'}
		<Alert kind="info">{m.archived_notice()}</Alert>
	{/if}
	{#if tank.access === 'viewer'}
		<Alert kind="info">{m.viewer_notice()}</Alert>
	{/if}
	{#if actionError}<Alert kind="error">{errorMessage(actionError)}</Alert>{/if}

	<TankDashboard tankId={tank.id} timeZone={tz ?? 'Asia/Ho_Chi_Minh'} />

	<section class="card" aria-labelledby="volumes">
		<h2 id="volumes">{m.tank_volumes()}</h2>
		<dl class="facts">
			<dt>{m.field_display_volume()}</dt>
			<dd>{m.volume_value({ value: dec(tank.display_volume_l) })}</dd>
			<dt>{m.field_sump_volume()}</dt>
			<dd>
				{tank.sump_volume_l ? m.volume_value({ value: dec(tank.sump_volume_l) }) : m.not_set()}
			</dd>
			<dt>{m.field_system_volume()}</dt>
			<dd>{m.volume_estimate({ value: dec(tank.system_volume_l) })}</dd>
			<dt>{m.field_dimensions()}</dt>
			<dd>
				{tank.dimensions_cm
					? m.dims_value({
							length: dec(tank.dimensions_cm.length),
							width: dec(tank.dimensions_cm.width),
							height: dec(tank.dimensions_cm.height)
						})
					: m.not_set()}
			</dd>
		</dl>
	</section>

	<section class="card" aria-labelledby="info">
		<h2 id="info">{m.tank_info()}</h2>
		<dl class="facts">
			<dt>{m.field_setup_date()}</dt>
			<dd>{tank.setup_date ? formatDate(tank.setup_date, locale) : m.not_set()}</dd>
			<dt>{m.field_salt_mix()}</dt>
			<dd>{tank.salt_mix ?? m.not_set()}</dd>
			<dt>{m.field_water_source()}</dt>
			<dd>{tank.water_source ?? m.not_set()}</dd>
			<dt>{m.label_created()}</dt>
			<dd><time datetime={tank.created_at}>{formatDateTime(tank.created_at, locale, tz)}</time></dd>
			{#if tank.archived_at}
				<dt>{m.label_archived_at()}</dt>
				<dd>
					<time datetime={tank.archived_at}>{formatDateTime(tank.archived_at, locale, tz)}</time>
				</dd>
			{/if}
		</dl>
		{#if tank.description}
			<h3>{m.field_description()}</h3>
			<p class="description">{tank.description}</p>
		{/if}
	</section>

	<nav class="card" aria-label={m.tank_sections()}>
		<ul class="sections">
			<li>
				<a href={resolve('/(app)/tanks/[id]/water-tests', { id: tank.id })}>{m.wp_title()}</a>
			</li>
			<li>
				<a href={resolve('/(app)/tanks/[id]/assessments', { id: tank.id })}>{m.ia_title()}</a>
			</li>
			<li>
				<a href={resolve('/(app)/tanks/[id]/water-changes', { id: tank.id })}
					>{m.tank_section_water_changes()}</a
				>
			</li>
			<li>
				<a href={resolve('/(app)/tanks/[id]/dosing', { id: tank.id })}>{m.tank_section_dosing()}</a>
			</li>
			<li>
				<a href={resolve('/(app)/tanks/[id]/livestock', { id: tank.id })}>{m.ls_title()}</a>
			</li>
			<li>
				<a href={resolve('/(app)/tanks/[id]/equipment', { id: tank.id })}>{m.eq_title()}</a>
			</li>
			<li>
				<a href={resolve('/(app)/tanks/[id]/maintenance', { id: tank.id })}
					>{m.tank_section_maintenance()}</a
				>
			</li>
		</ul>
	</nav>

	<div class="actions">
		{#if tank.access === 'owner'}
			{#if tank.status !== 'archived'}
				<a class="button" href={resolve('/(app)/tanks/[id]/edit', { id: tank.id })}
					>{m.tank_edit()}</a
				>
			{/if}
			<a class="button secondary" href={resolve('/(app)/tanks/[id]/sharing', { id: tank.id })}
				>{m.tank_sharing()}</a
			>
			{#if tank.status !== 'archived'}
				<button type="button" class="secondary" onclick={() => (confirmArchive = true)}
					>{m.tank_archive()}</button
				>
			{/if}
		{:else}
			<button type="button" class="secondary" onclick={() => (confirmLeave = true)}
				>{m.leave_tank()}</button
			>
		{/if}
	</div>

	<ConfirmDialog
		bind:open={confirmArchive}
		title={m.archive_confirm_title()}
		confirmLabel={m.archive_confirm()}
		danger
		{busy}
		onconfirm={archive}
	>
		<p>{m.archive_confirm_body()}</p>
	</ConfirmDialog>
	<ConfirmDialog
		bind:open={confirmLeave}
		title={m.leave_confirm_title()}
		confirmLabel={m.leave_tank()}
		danger
		{busy}
		onconfirm={leave}
	>
		<p>{m.leave_confirm_body()}</p>
	</ConfirmDialog>
{/if}

<style>
	.meta {
		display: flex;
		flex-wrap: wrap;
		gap: 0.4rem;
	}
	.description {
		white-space: pre-line;
	}
	h3 {
		font-size: 1rem;
		margin: 1rem 0 0.25rem;
	}
	.sections {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem 1.25rem;
	}
</style>
