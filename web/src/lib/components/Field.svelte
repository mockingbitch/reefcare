<!--
	Labelled text input or textarea with hint and error. The error is announced through
	aria-describedby and shown with an icon, never by color alone.
-->
<script lang="ts">
	import type { HTMLInputAttributes } from 'svelte/elements';
	import { m } from '#lib/paraglide/messages.js';

	type Props = Omit<HTMLInputAttributes, 'value'> & {
		label: string;
		value?: string;
		error?: string;
		hint?: string;
		optional?: boolean;
		multiline?: boolean;
	};

	let {
		label,
		value = $bindable(''),
		error,
		hint,
		optional = false,
		multiline = false,
		...rest
	}: Props = $props();

	const id = $props.id();
	const describedBy = $derived(
		[hint ? `${id}-hint` : '', error ? `${id}-error` : ''].filter(Boolean).join(' ') || undefined
	);
</script>

<div class="field">
	<label for={id}>
		{label}
		{#if optional}<span class="muted">({m.optional()})</span>{/if}
	</label>
	{#if multiline}
		<textarea
			{id}
			name={rest.name}
			rows="4"
			bind:value
			aria-invalid={error ? 'true' : undefined}
			aria-describedby={describedBy}></textarea>
	{:else}
		<input
			{id}
			{...rest}
			bind:value
			aria-invalid={error ? 'true' : undefined}
			aria-describedby={describedBy}
		/>
	{/if}
	{#if hint}<p id="{id}-hint" class="hint">{hint}</p>{/if}
	{#if error}<p id="{id}-error" class="field-error">
			<span aria-hidden="true">⚠</span>
			{error}
		</p>{/if}
</div>
