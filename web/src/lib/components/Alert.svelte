<!--
	Status message. Errors use role="alert" so screen readers announce them at once; other
	kinds are polite. Each kind has an icon and text, not just a color.
-->
<script lang="ts">
	import type { Snippet } from 'svelte';

	type Props = { kind?: 'error' | 'success' | 'info' | 'warning'; children: Snippet };
	let { kind = 'info', children }: Props = $props();

	const icons = { error: '⚠', warning: '⚠', success: '✓', info: 'ℹ' };
</script>

<div class="alert alert-{kind}" role={kind === 'error' ? 'alert' : 'status'}>
	<span class="alert-icon" aria-hidden="true">{icons[kind]}</span>
	<div>{@render children()}</div>
</div>
