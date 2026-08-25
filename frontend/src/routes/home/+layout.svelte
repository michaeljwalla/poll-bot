<script lang="ts">
	import { page } from '$app/state';
	import { fetchContent } from '$lib/content/content';

	let { children } = $props();

	const pages: Record<string, string> = {
		'/home': 'Home',
		'/home/database': 'Modify Database',
		'/home/aliases': 'Modify Aliases',
		'/home/self': 'Login Status'
	};
	const title = $derived(pages[page.route.id ?? ''] ?? 'Unknown');
</script>

<main>
	<h1>{title}</h1>
	{@render children()}
</main>
<aside class="stacked">
	<h3>Navigation</h3>
	<nav>
		{#each Object.entries(pages) as [route, title], i}
			{#if page.route.id == route}
				<span>{title}</span>
			{:else}
				<a href={route}>{title}</a>
			{/if}
			{#if i < Object.keys(pages).length - 1}
				{' - '}
			{/if}
		{/each}
	</nav>
</aside>
