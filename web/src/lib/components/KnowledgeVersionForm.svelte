<!--
	Edits a knowledge draft. Structured lists are edited as text, one item per line
	(#lib/knowledge.js). The server validates everything; this only shapes the request.
-->
<script lang="ts">
	import { untrack } from 'svelte';
	import type {
		ApiError,
		KnowledgeSource,
		KnowledgeVersion,
		KnowledgeVersionInput
	} from '#lib/api/client.js';
	import Alert from '#lib/components/Alert.svelte';
	import Field from '#lib/components/Field.svelte';
	import SelectField from '#lib/components/SelectField.svelte';
	import { errorMessage, fieldErrors } from '#lib/errors.js';
	import {
		claimClasses,
		claimLabel,
		structuredToText,
		tags,
		textToStructured
	} from '#lib/knowledge.js';
	import { m } from '#lib/paraglide/messages.js';

	type Props = {
		initial?: KnowledgeVersion | null;
		sources: KnowledgeSource[];
		submitLabel: string;
		save: (input: KnowledgeVersionInput) => Promise<ApiError | null>;
	};
	let { initial = null, sources, submitLabel, save }: Props = $props();
	// The form starts from `initial`; callers remount it ({#key}) to load another version.
	const start = untrack(() => initial);

	let title = $state(start?.title ?? '');
	let summary = $state(start?.summary ?? '');
	let body = $state(start?.body_md ?? '');
	let claim = $state<string>(start?.claim_class ?? 'unverified');
	let parameters = $state((start?.parameters ?? []).join(', '));
	let symptoms = $state((start?.symptoms ?? []).join(', '));
	let lists = $state(structuredToText(start?.structured));
	let picked = $state<string[]>((start?.sources ?? []).map((s) => s.id));
	const notes = new Map((start?.sources ?? []).map((s) => [s.id, s.note ?? null]));
	let errors = $state<Record<string, string>>({});
	let error = $state<ApiError | null>(null);
	let listErrors = $state<string[]>([]);
	let busy = $state(false);

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		const parsed = textToStructured(lists);
		listErrors = parsed.errors;
		if (listErrors.length) return;
		busy = true;
		const err = await save({
			title,
			summary,
			body_md: body,
			claim_class: claim as KnowledgeVersionInput['claim_class'],
			parameters: tags(parameters),
			symptoms: tags(symptoms),
			structured: parsed.structured,
			// Keep each source's note (set by the API); the form only picks sources.
			sources: picked.map((id) => ({ source_id: id, note: notes.get(id) ?? null }))
		});
		busy = false;
		errors = fieldErrors(err);
		error = err && !Object.keys(errors).length ? err : null;
	}
</script>

<form onsubmit={submit}>
	{#if error}<Alert kind="error">{errorMessage(error)}</Alert>{/if}
	{#if listErrors.length}
		<Alert kind="error">{m.kb_list_errors({ lines: listErrors.join(', ') })}</Alert>
	{/if}
	<Field
		label={m.kb_field_title()}
		bind:value={title}
		error={errors.title}
		maxlength={200}
		required
	/>
	<Field
		label={m.kb_field_summary()}
		bind:value={summary}
		error={errors.summary}
		multiline
		required
	/>
	<Field label={m.kb_field_body()} bind:value={body} error={errors.body_md} multiline required />
	<SelectField
		label={m.kb_field_claim()}
		name="claim_class"
		bind:value={claim}
		error={errors.claim_class}
		options={claimClasses.map((c) => ({ value: c, label: claimLabel(c) }))}
	/>
	<Field
		label={m.kb_field_parameters()}
		hint={m.kb_field_parameters_hint()}
		bind:value={parameters}
		error={errors.parameters ??
			Object.entries(errors).find(([k]) => k.startsWith('parameters.'))?.[1]}
		optional
	/>
	<Field
		label={m.kb_field_symptoms()}
		hint={m.kb_field_symptoms_hint()}
		bind:value={symptoms}
		error={errors.symptoms ?? Object.entries(errors).find(([k]) => k.startsWith('symptoms.'))?.[1]}
		optional
	/>
	<p class="muted">{m.kb_lists_hint()}</p>
	<Field label={m.kb_safety_warnings()} bind:value={lists.safety_warnings} multiline optional />
	<Field label={m.kb_verifications()} bind:value={lists.verifications} multiline optional />
	<Field label={m.kb_recommendations()} bind:value={lists.recommendations} multiline optional />
	<Field label={m.kb_contraindications()} bind:value={lists.contraindications} multiline optional />
	{#each Object.entries(errors).filter(([k]) => k.startsWith('structured')) as [k, msg] (k)}
		<p class="error">{k}: {msg}</p>
	{/each}

	<fieldset>
		<legend>{m.kb_pick_sources()}</legend>
		{#if sources.length === 0}
			<p class="muted">{m.kb_no_sources()}</p>
		{/if}
		{#each sources as s (s.id)}
			<label class="check">
				<input type="checkbox" value={s.id} bind:group={picked} />
				{s.title}{#if s.publisher}
					· {s.publisher}{/if}
			</label>
		{/each}
		{#if errors.sources}<p class="error">{errors.sources}</p>{/if}
	</fieldset>
	<button type="submit" disabled={busy}>{submitLabel}</button>
</form>

<style>
	.error {
		color: var(--danger, #b00020);
	}
	fieldset {
		margin: 1rem 0;
	}
</style>
