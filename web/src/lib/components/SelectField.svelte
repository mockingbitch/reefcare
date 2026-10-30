<script lang="ts">
	type Option = { value: string; label: string };
	type Props = {
		label: string;
		name: string;
		options: Option[];
		value?: string;
		error?: string;
		hint?: string;
	};

	let { label, name, options, value = $bindable(''), error, hint }: Props = $props();

	const id = $props.id();
	const describedBy = $derived(
		[hint ? `${id}-hint` : '', error ? `${id}-error` : ''].filter(Boolean).join(' ') || undefined
	);
</script>

<div class="field">
	<label for={id}>{label}</label>
	<select
		{id}
		{name}
		bind:value
		aria-invalid={error ? 'true' : undefined}
		aria-describedby={describedBy}
	>
		{#each options as o (o.value)}
			<option value={o.value}>{o.label}</option>
		{/each}
	</select>
	{#if hint}<p id="{id}-hint" class="hint">{hint}</p>{/if}
	{#if error}<p id="{id}-error" class="field-error">
			<span aria-hidden="true">⚠</span>
			{error}
		</p>{/if}
</div>
