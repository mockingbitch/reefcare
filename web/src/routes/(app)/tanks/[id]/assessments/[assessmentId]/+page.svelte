<script lang="ts">
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import {
		api,
		type ApiError,
		type Assessment,
		type AssessmentFeedback,
		type Tank
	} from '#lib/api/client.js';
	import Alert from '#lib/components/Alert.svelte';
	import ConfirmDialog from '#lib/components/ConfirmDialog.svelte';
	import Field from '#lib/components/Field.svelte';
	import {
		assessmentText,
		confidenceLabel,
		evidenceCount,
		orderedWarnings,
		severityIcon,
		severityLabel,
		triggerLabel
	} from '#lib/assessments.js';
	import { errorMessage, fieldErrors } from '#lib/errors.js';
	import { formatDateTime } from '#lib/format.js';
	import { claimLabel } from '#lib/knowledge.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import { session } from '#lib/session.svelte.js';

	const id = $derived(page.params.id ?? '');
	const assessmentId = $derived(page.params.assessmentId ?? '');
	const locale = getLocale();
	const tz = $derived(session.user?.timezone ?? 'Asia/Ho_Chi_Minh');
	const say = (t: Parameters<typeof assessmentText>[0]) => assessmentText(t, locale);

	let a = $state<Assessment | null>(null);
	let tank = $state<Tank | null>(null);
	let loading = $state(true);
	let error = $state<ApiError | null>(null);

	let rating = $state<AssessmentFeedback['rating'] | ''>('');
	let note = $state('');
	let feedbackSaved = $state(false);
	let feedbackError = $state<ApiError | null>(null);

	let retractOpen = $state(false);
	let reason = $state('');
	let reasonError = $state('');
	let retractError = $state<ApiError | null>(null);
	let busy = $state(false);

	async function load() {
		loading = true;
		error = null;
		const r = await api<Assessment>('GET', `/assessments/${assessmentId}`);
		if (!r.ok) {
			loading = false;
			return void (error = r.error);
		}
		// The assessment must belong to the tank in the address (actions depend on its access).
		if (r.data.tank_id !== id) {
			loading = false;
			return void (error = { status: 404, code: 'not_found', message: 'not found', details: [] });
		}
		const t = await api<Tank>('GET', `/tanks/${r.data.tank_id}`);
		loading = false;
		if (!t.ok) return void (error = t.error);
		a = r.data;
		tank = t.data;
		rating = r.data.my_feedback?.rating ?? '';
		note = r.data.my_feedback?.outcome_note ?? '';
	}
	$effect(() => {
		void assessmentId;
		load();
	});

	async function sendFeedback(e: SubmitEvent) {
		e.preventDefault();
		if (!rating) return;
		feedbackSaved = false;
		feedbackError = null;
		busy = true;
		const r = await api<AssessmentFeedback>('PUT', `/assessments/${assessmentId}/feedback`, {
			rating,
			outcome_note: note.trim() || null
		});
		busy = false;
		if (!r.ok) return void (feedbackError = r.error);
		feedbackSaved = true;
	}

	async function retract() {
		reasonError = '';
		retractError = null;
		busy = true;
		const r = await api<Assessment>('POST', `/assessments/${assessmentId}/retract`, {
			reason: reason.trim()
		});
		busy = false;
		if (!r.ok) {
			reasonError = fieldErrors(r.error).reason ?? '';
			if (!reasonError) retractError = r.error;
			return;
		}
		retractOpen = false;
		a = r.data;
	}
</script>

<svelte:head><title>{m.ia_title()} · {m.app_name()}</title></svelte:head>

<p><a href={resolve('/(app)/tanks/[id]/assessments', { id })}>← {m.ia_back()}</a></p>

{#if loading && !a}
	<p role="status">{m.loading()}</p>
{:else if error}
	<Alert kind="error">
		{errorMessage(error)}
		<button type="button" class="secondary" onclick={load}>{m.retry()}</button>
	</Alert>
{:else if a}
	<h1>
		{m.ia_title()} ·
		<time datetime={a.created_at}>{formatDateTime(a.created_at, locale, tz)}</time>
	</h1>
	<p class="muted">{triggerLabel(a.trigger)} · {confidenceLabel(a.confidence_label)}</p>
	<p class="muted small">{m.ia_conf_basis()}</p>
	<Alert kind="info">{m.ia_intro()}</Alert>
	{#if a.provisional}<p class="muted small">{m.ia_provisional()}</p>{/if}
	{#if a.status === 'retracted'}
		<Alert kind="warning">{m.ia_retracted_reason({ reason: a.retract_reason ?? '' })}</Alert>
	{/if}

	{#if !a.findings?.length}
		<p>{m.ia_no_findings()}</p>
	{:else}
		{#each a.findings as f, i (i)}
			<article class="finding sev-{f.severity}" aria-labelledby="f-{i}">
				<h2 id="f-{i}">
					<span class="badge"
						><span aria-hidden="true">{severityIcon(f.severity)}</span>
						{severityLabel(f.severity)}</span
					>
					{say(f.summary)}
				</h2>
				<p class="muted small">{confidenceLabel(f.confidence_label)}</p>

				{#if f.safety_warnings.length}
					<ul class="warnings" aria-label={m.ia_safety()}>
						{#each orderedWarnings(f) as w, j (j)}
							<li class:urgent={w.urgent}>
								<span aria-hidden="true">{w.urgent ? '⛔' : '⚠'}</span>
								{say(w.text)}
							</li>
						{/each}
					</ul>
				{/if}

				{#if f.observations.length}
					<h3>{m.ia_observations()}</h3>
					<ul>
						{#each f.observations as o, j (j)}<li>{say(o.text)}</li>{/each}
					</ul>
					<p class="muted small">{m.ia_evidence({ count: String(evidenceCount(f)) })}</p>
				{/if}

				{#if f.hypotheses.length}
					<h3>{m.ia_hypotheses()}</h3>
					<ul>
						{#each f.hypotheses as h, j (j)}
							<li>
								{say(h.text)}
								{#if h.claim_class}<span class="muted small"
										>— {m.kb_claim({ label: claimLabel(h.claim_class) })}</span
									>{/if}
								{#if h.correlation}<span class="muted small">— {m.ia_correlation()}</span>{/if}
								<span class="muted small"
									>({m.ia_supporting({
										count: String(h.supporting.length)
									})}{#if h.contradicting.length},
										{m.ia_contradicting({ count: String(h.contradicting.length) })}{/if})</span
								>
							</li>
						{/each}
					</ul>
				{/if}

				{#if f.alternative_explanations.length}
					<h3>{m.ia_alternatives()}</h3>
					<ul>
						{#each f.alternative_explanations as h, j (j)}
							<li>
								{say(h.text)}
								{#if h.claim_class}<span class="muted small"
										>— {m.kb_claim({ label: claimLabel(h.claim_class) })}</span
									>{/if}
							</li>
						{/each}
					</ul>
				{/if}

				{#if f.missing_information.length}
					<h3>{m.ia_missing()}</h3>
					<ul>
						{#each f.missing_information as x, j (j)}<li>{say(x.text)}</li>{/each}
					</ul>
				{/if}

				{#if f.recommended_verifications.length}
					<h3>{m.ia_verifications()}</h3>
					<ul>
						{#each f.recommended_verifications as x, j (j)}<li>{say(x.text)}</li>{/each}
					</ul>
				{/if}

				{#if f.knowledge_references.length}
					<h3>{m.ia_knowledge()}</h3>
					<ul>
						{#each f.knowledge_references as k (k.version_id)}
							<li>
								<a href={resolve('/(app)/knowledge/[slug]', { slug: k.slug })}>{k.title}</a>
							</li>
						{/each}
					</ul>
				{/if}
			</article>
		{/each}
	{/if}

	<section class="card" aria-labelledby="fb">
		<h2 id="fb">{m.ia_feedback_title()}</h2>
		<form onsubmit={sendFeedback}>
			<fieldset>
				<legend class="visually-hidden">{m.ia_feedback_title()}</legend>
				{#each [['useful', m.ia_rating_useful()], ['not_useful', m.ia_rating_not_useful()], ['wrong', m.ia_rating_wrong()]] as [value, label] (value)}
					<label class="check">
						<input
							type="radio"
							name="rating"
							{value}
							bind:group={rating}
							onchange={() => (feedbackSaved = false)}
						/>
						{label}
					</label>
				{/each}
			</fieldset>
			<Field label={m.ia_outcome_note()} bind:value={note} multiline maxlength={2000} optional />
			<button type="submit" disabled={!rating || busy}>{m.ia_send_feedback()}</button>
			{#if feedbackSaved}<Alert kind="success">{m.ia_feedback_saved()}</Alert>{/if}
			{#if feedbackError}<Alert kind="error">{errorMessage(feedbackError)}</Alert>{/if}
		</form>
	</section>

	{#if tank?.access === 'owner' && tank.status !== 'archived' && a.status !== 'retracted'}
		<div class="actions">
			<button type="button" class="secondary" onclick={() => (retractOpen = true)}
				>{m.ia_retract()}</button
			>
		</div>
	{/if}
{/if}

<ConfirmDialog
	bind:open={retractOpen}
	title={m.ia_retract()}
	confirmLabel={m.ia_retract()}
	danger
	{busy}
	onconfirm={retract}
>
	<p>{m.ia_retract_hint()}</p>
	<Field
		label={m.ia_retract_reason()}
		bind:value={reason}
		error={reasonError}
		maxlength={500}
		required
	/>
	{#if retractError}<Alert kind="error">{errorMessage(retractError)}</Alert>{/if}
</ConfirmDialog>

<style>
	.finding {
		border: 1px solid var(--border, #dde);
		border-left-width: 6px;
		border-radius: 6px;
		padding: 0.75rem 1rem;
		margin: 1rem 0;
	}
	.sev-urgent {
		border-left-color: #b00020;
	}
	.sev-warning {
		border-left-color: #b26a00;
	}
	.sev-notice {
		border-left-color: #2a5db0;
	}
	h2 {
		font-size: 1.1rem;
		margin: 0;
	}
	h3 {
		font-size: 1rem;
		margin: 0.75rem 0 0.25rem;
	}
	.warnings {
		list-style: none;
		padding: 0;
	}
	.warnings .urgent {
		font-weight: 600;
	}
	.small {
		font-size: 0.9rem;
	}
	.actions {
		margin-top: 1rem;
	}
	fieldset {
		border: 0;
		padding: 0;
		display: flex;
		gap: 1rem;
		flex-wrap: wrap;
	}
</style>
