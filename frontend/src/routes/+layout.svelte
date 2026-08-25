<script lang="ts">
	import { goto } from '$app/navigation';
	import favicon from '$lib/assets/favicon.svg';

	import { onMount } from 'svelte';
	import { tryCredentials, syncAuthExpiry } from '$lib/login/login.svelte.js';

	let { children, data } = $props();

	// invalidateAll() after a password login re-runs the layout load, so track data.auth
	// rather than reading it once on mount.
	$effect(() => {
		syncAuthExpiry(data.auth);
	});

	onMount(async () => {
		if ((await tryCredentials(data.auth)).success) {
			goto('/home');
		} else {
			goto('/login');
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
