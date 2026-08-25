<script lang="ts">
	import { Duration } from 'luxon';
	import { onMount } from 'svelte';
	import { fetchAliases } from '$lib/content/content';
	import { StatusCodes } from 'http-status-codes';
	import { goto } from '$app/navigation';
	import InputField from './InputField.svelte';

	let aliases = $state<Record<string, string>>({});

	// Rows carry their own key: the id is user-editable, so it cannot identify a row.
	type Addition = { key: number; id: string; alias: string };
	let nextKey = 0;
	let additions = $state<Addition[]>([]);
	let _additions = $derived(
		additions.filter((a) => a.id.trim() !== '' && a.alias.trim() !== '')
	);
	let removals = $state<Record<string, null>>({});
	let changes = $state<Record<string, string>>({});

	let time = $state(Date.now());
	let lastUpdated = $state<number>(0);
	const expiryFormatted = $derived.by(() => {
		if (lastUpdated == 0) {
			return '(in progress)';
		}
		if (time - lastUpdated <= 5000) {
			return 'now';
		}
		return Duration.fromMillis(time - lastUpdated).toFormat('hh:mm:ss ago');
	});

	async function loadAliases() {
		const currentAliases = await fetchAliases();
		if (currentAliases === null) return;
		else if (currentAliases === StatusCodes.UNAUTHORIZED) {
			goto('/login');
			return;
		}
		aliases = currentAliases;
		lastUpdated = Date.now();
	}

	type blurReturn = 'red' | 'green' | 'blue' | undefined;
	function showDiff(e: any, old: string, cur: string): blurReturn {
		return 'red';
	}

	function onManualModId(row: Addition) {
		return (e: any, old: string, cur: string): blurReturn => {
			row.id = cur;
			return 'green';
		};
	}
	function onManualModAlias(row: Addition) {
		return (e: any, old: string, cur: string): blurReturn => {
			row.alias = cur;
			return 'green';
		};
	}
	function onManualModDelete(row: Addition) {
		return (): blurReturn => {
			additions = additions.filter((a) => a.key !== row.key);
			return undefined;
		};
	}
	$effect(() => {
		loadAliases();
		//
		const tick = setInterval(() => (time = Date.now()), 1000);
		return () => clearInterval(tick);
	});
</script>

<p>You can modify users' names here, or through the bot with /alias.</p>
<h2>Last updated: {expiryFormatted}</h2>
<ul class="centered stacked">
	<InputField lhs={{ value: 'id', disabled: true }} rhs={{ value: 'alias', disabled: true }} />
	{#each Object.entries(aliases) as [id, alias] (id)}
		<InputField lhs={{ value: id, disabled: true }} rhs={{ value: alias, onblur: showDiff }} />
	{/each}
	{#each additions as row (row.key)}
		<InputField
			lhs={{ value: row.id, onblur: onManualModId(row) }}
			rhs={{ value: row.alias, onblur: onManualModAlias(row) }}
			ondel={onManualModDelete(row)}
		/>
	{/each}
	<button
		onclick={() => {
			additions.push({ key: nextKey++, id: String(), alias: String() });
		}}>New Alias</button
	>
</ul>

<h2>Outgoing changes</h2>
<div class="rowed">
	<div class="fill stacked">
		<h2>Modified</h2>
		<ul>
			{#if Object.keys(changes).length}
				{#each Object.entries(changes) as [id, to]}
					<li>{id} --> {to}</li>
				{/each}
			{:else}
				<li>No changes yet.</li>
			{/if}
		</ul>
	</div>
	<div class="fill stacked">
		<h2>Added</h2>
		<ul>
			{#if _additions.length}
				{#each _additions as add (add.key)}
					<li>{add.alias} ({add.id})</li>
				{/each}
			{:else}
				<li>None added.</li>
			{/if}
		</ul>
	</div>
	<div class="fill stacked">
		<h2>Removed</h2>
		<ul>
			{#if Object.keys(removals).length}
				{#each Object.entries(removals) as [id, _]}
					<li>{id}</li>
				{/each}
			{:else}
				<li>None removed.</li>
			{/if}
		</ul>
	</div>
</div>
