<!--
	Modal confirmation built on <dialog>: showModal() traps focus, Escape closes it and
	focus returns to the opener. Extra fields (e.g. a password) go in `children`.
-->
<script lang="ts">
	import type { Snippet } from 'svelte';
	import { m } from '#lib/paraglide/messages.js';

	type Props = {
		open: boolean;
		title: string;
		confirmLabel: string;
		danger?: boolean;
		busy?: boolean;
		onconfirm: () => void;
		children?: Snippet;
	};

	let {
		open = $bindable(false),
		title,
		confirmLabel,
		danger = false,
		busy = false,
		onconfirm,
		children
	}: Props = $props();

	let dialog: HTMLDialogElement | undefined = $state();
	const titleId = $props.id();

	$effect(() => {
		if (!dialog) return;
		if (open && !dialog.open) dialog.showModal();
		if (!open && dialog.open) dialog.close();
	});
</script>

<dialog bind:this={dialog} aria-labelledby={titleId} onclose={() => (open = false)}>
	<form
		method="dialog"
		onsubmit={(e) => {
			e.preventDefault();
			onconfirm();
		}}
	>
		<h2 id={titleId}>{title}</h2>
		{@render children?.()}
		<div class="actions">
			<button type="button" class="secondary" onclick={() => (open = false)}>{m.cancel()}</button>
			<button type="submit" class={danger ? 'danger' : ''} disabled={busy}>{confirmLabel}</button>
		</div>
	</form>
</dialog>
