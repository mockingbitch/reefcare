<!--
	A water-change estimate with its assumptions and flags, which the API contract requires
	clients to show alongside the number. Flags are warnings, never blocking.
-->
<script lang="ts">
	import type { WaterChangeEstimate } from '#lib/api/client.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import { assumptionLabel, estimateSummary, flagLabel } from '#lib/water-changes.js';

	let { estimate }: { estimate: WaterChangeEstimate } = $props();
	const locale = getLocale();
</script>

<div class="estimate">
	<p>{estimateSummary(estimate, locale)}</p>
	{#each estimate.flags as flag (flag)}
		<p class="flag"><span aria-hidden="true">⚠</span> {flagLabel(flag)}</p>
	{/each}
	<details>
		<summary>{m.wc_estimate_assumptions()}</summary>
		<ul>
			{#each estimate.assumptions as a (a)}
				<li>{assumptionLabel(a)}</li>
			{/each}
		</ul>
	</details>
</div>

<style>
	.estimate p {
		margin: 0.15rem 0;
	}
	.flag {
		font-weight: 600;
	}
	details {
		font-size: 0.9rem;
	}
	ul {
		margin: 0.25rem 0 0;
		padding-left: 1.25rem;
	}
</style>
