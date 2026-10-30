<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { m } from '#lib/paraglide/messages.js';
</script>

<h1>{page.status === 404 ? m.not_found_title() : m.error_title()}</h1>
{#if page.status === 404}
	<p>{m.not_found_body()}</p>
{/if}
<p><a href={resolve('/')}>{m.go_home()}</a></p>
