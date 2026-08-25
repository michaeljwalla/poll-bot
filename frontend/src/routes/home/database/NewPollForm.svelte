<script lang="ts">
	import type { PollAddition } from '$lib/content/content';

	type Props = {
		poll: PollAddition;
		disabled?: boolean;
		onRemovePoll: () => void;
	};
	const { poll, disabled = false, onRemovePoll }: Props = $props();

	// Voter rows carry their own key: the voter field is user-editable text
	// (an id or an alias), so it cannot identify a row.
	let nextVoteKey = 0;

	const CHOICES = ['N/A', '⭐', '⭐⭐', '⭐⭐⭐', '⭐⭐⭐⭐', '⭐⭐⭐⭐⭐'];

	function addVoter() {
		poll.votes.push({ key: nextVoteKey++, voter: '', choice: 0 });
	}
	function removeVoter(key: number) {
		poll.votes = poll.votes.filter((v) => v.key !== key);
	}
</script>

<li class="draft green">
	<div class="draft-head">
		<input placeholder="Poll ID (snowflake)" bind:value={poll.id} {disabled} class="mono" />
		<input placeholder="Title" bind:value={poll.title} {disabled} />
		<button {disabled} onclick={onRemovePoll}>Remove Poll</button>
	</div>
	<ul class="voters">
		{#each poll.votes as vote (vote.key)}
			<li class="voter-row">
				<input placeholder="Voter (id or alias)" bind:value={vote.voter} {disabled} />
				<select bind:value={vote.choice} {disabled}>
					{#each CHOICES as label, i (i)}
						<option value={i}>{label}</option>
					{/each}
				</select>
				<button {disabled} onclick={() => removeVoter(vote.key)}>X</button>
			</li>
		{/each}
	</ul>
	<button {disabled} onclick={addVoter}>Add Voter</button>
</li>

<style>
	.draft {
		display: flex;
		flex-direction: column;
		gap: 0.6rem;
		align-items: flex-start;
		padding: 0.75rem;
		border-radius: 4px;
	}
	.draft-head {
		display: grid;
		grid-template-columns: 21ch minmax(14rem, 1fr) auto;
		gap: 0.75rem;
		align-items: center;
		width: 100%;
	}
	.voters {
		list-style: none;
		margin: 0;
		padding: 0 0 0 1.5rem;
		display: flex;
		flex-direction: column;
		gap: 0.4rem;
		width: 100%;
	}
	/* same tracks on every voter row so the choice selects line up */
	.voter-row {
		display: grid;
		grid-template-columns: minmax(14rem, 24ch) auto 3rem;
		gap: 0.75rem;
		align-items: center;
	}
	.mono {
		font-family: monospace;
	}
	.green {
		background-color: lightgreen;
	}
</style>
