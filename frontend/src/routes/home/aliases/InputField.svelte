<script lang="ts">
	import type { EventHandler } from 'svelte/elements';

	type colorReturn = 'red' | 'green' | 'blue' | undefined;
	type PropSide = {
		disabled?: boolean;
		value: string;
		onblur?: (
			event: FocusEvent & { currentTarget: EventTarget & HTMLInputElement },
			old: string,
			cur: string,
			left: string,
			right: string
		) => colorReturn;
	};
	type Props = {
		lhs: PropSide;
		rhs: PropSide;
		ondel?: (
			event: MouseEvent & { currentTarget: EventTarget & HTMLButtonElement },
			active: boolean,
			left: string,
			right: string
		) => colorReturn;
	};

	const { lhs, rhs, ondel }: Props = $props();

	let oldLeft = $derived(lhs.value);
	let oldRight = $derived(rhs.value);

	// svelte-ignore state_referenced_locally
	let curLeft = $state(oldLeft);
	// svelte-ignore state_referenced_locally
	let curRight = $state(oldRight);
	let color = $state<'red' | 'green' | 'blue' | undefined>();

	let delSymbol = $state('X');
</script>

<li class="centered rowed">
	<input
		name="lhs"
		onblur={(e) => {
			if (!lhs.onblur) return;
			color = lhs.onblur(e, oldLeft, curLeft, curLeft, curRight);
			oldLeft = curLeft;
		}}
		bind:value={curLeft}
		disabled={lhs.disabled}
		class="centered {color}"
	/>
	<input
		name="rhs"
		onblur={(e) => {
			if (!rhs.onblur) return;
			color = rhs.onblur(e, oldRight, curRight, curLeft, curRight);
			oldRight = curRight;
		}}
		bind:value={curRight}
		disabled={rhs.disabled}
		class="centered {color}"
	/>
	{#if ondel}
		<button
			class={color}
			onclick={(e) => {
				color = ondel(e, delSymbol === 'X', curLeft, curRight);
				delSymbol = color === 'red' ? '-' : 'X';
			}}>{delSymbol}</button
		>
	{/if}
</li>

<style>
	.red {
		background-color: lightcoral;
	}
	.blue {
		background-color: cornflowerblue;
	}
	.green {
		background-color: lightgreen;
	}
</style>
