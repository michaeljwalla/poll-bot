<script lang="ts">
	import { Duration } from 'luxon';
	import { onMount } from 'svelte';
	import { getAuthExpiry } from '$lib/login/login.svelte';

	let now = $state(Date.now());
	onMount(() => {
		const tick = setInterval(() => (now = Date.now()), 1000);
		return () => clearInterval(tick);
	});

	const remaining = $derived(Math.max(0, (getAuthExpiry() ?? 0) - now));
	const expiryFormatted = $derived(
		Duration.fromMillis(remaining).toFormat("m 'minutes until logout'")
	);
</script>

<p>{expiryFormatted}</p>
