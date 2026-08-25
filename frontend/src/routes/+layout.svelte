<script lang="ts">
	import { goto } from '$app/navigation';
	import { base } from '$app/paths';
	import favicon from '$lib/assets/favicon.svg';

	import { onMount } from 'svelte';
	import { tryCredentials } from '$lib/login/login.svelte.js';

	let { children } = $props();

	// No server load hands the session down any more (the deployment is a static
	// bundle), so the cookie is validated by asking the API directly.
	onMount(async () => {
		if ((await tryCredentials()).success) {
			goto(`${base}/home`);
		} else {
			goto(`${base}/login`);
		}
	});
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

{@render children()}

<style>
	:global(.stacked) {
		display: flex;
		justify-content: center;
		flex-direction: column;
	}
	:global(.centered) {
		align-items: center;
		text-align: center;
	}
	:global(.rowed) {
		display: flex;
		flex-direction: row;
	}
	:global(.fill) {
		flex-grow: 1;
	}
</style>
