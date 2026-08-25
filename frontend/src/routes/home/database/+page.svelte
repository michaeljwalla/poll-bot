<script lang="ts">
	import { Duration } from 'luxon';
	import {
		fetchContent,
		fetchAliases,
		modPolls,
		type PollRow,
		type PollAddition
	} from '$lib/content/content';
	import { StatusCodes } from 'http-status-codes';
	import { goto } from '$app/navigation';
	import { base } from '$app/paths';
	import NewPollForm from './NewPollForm.svelte';

	let blockChanges = $state(false);

	let polls = $state<PollRow[]>([]);
	let aliases = $state<Record<string, string>>({});

	// Poll drafts (manual additions) carry their own key: the poll id is
	// user-editable, so it cannot identify a row.
	let nextKey = 0;
	let additions = $state<PollAddition[]>([]);

	// Staged edits, keyed by the poll's (stable, read-only) id.
	let titles = $state<Record<string, string>>({});
	let actives = $state<Record<string, boolean>>({});
	let removals = $state<Record<string, null>>({});

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

	// Newest-first: poll ids are Discord snowflakes (or synthesized ones for
	// manual polls), which sort numerically by creation time.
	const sortedPolls = $derived(
		[...polls].sort((a, b) => {
			try {
				const bi = BigInt(b.id);
				const ai = BigInt(a.id);
				return bi > ai ? 1 : bi < ai ? -1 : 0;
			} catch {
				return 0;
			}
		})
	);
	const pollsById = $derived(new Map(polls.map((p) => [p.id, p])));

	async function loadContent() {
		const [contentResult, aliasResult] = await Promise.all([fetchContent(), fetchAliases()]);
		if (contentResult === StatusCodes.UNAUTHORIZED || aliasResult === StatusCodes.UNAUTHORIZED) {
			goto(`${base}/login`);
			return;
		}
		if (contentResult !== null) {
			polls = contentResult.Rows;
		}
		if (aliasResult !== null) {
			aliases = aliasResult;
		}
		lastUpdated = Date.now();
	}

	function rowColor(row: PollRow): 'red' | 'blue' | undefined {
		if (row.id in removals) return 'red';
		if (row.id in titles || row.id in actives) return 'blue';
		return undefined;
	}

	function onTitleBlur(row: PollRow, cur: string) {
		if (cur === row.topic) delete titles[row.id];
		else titles[row.id] = cur;
	}
	function onActiveChange(row: PollRow, cur: boolean) {
		if (cur === row.active) delete actives[row.id];
		else actives[row.id] = cur;
	}
	function onRemoveToggle(row: PollRow) {
		if (row.id in removals) delete removals[row.id];
		else removals[row.id] = null;
	}

	function addNewPoll() {
		additions.push({ key: nextKey++, id: '', title: '', votes: [] });
	}
	function removeAddition(key: number) {
		additions = additions.filter((a) => a.key !== key);
	}

	function buildAliasToIds(): Map<string, string[]> {
		const map = new Map<string, string[]>();
		for (const [id, alias] of Object.entries(aliases)) {
			const list = map.get(alias);
			if (list) list.push(id);
			else map.set(alias, [id]);
		}
		return map;
	}

	// Resolution order for a typed voter value:
	// 1. it is a key of the alias record -> use it as the id
	// 2. otherwise it matches an alias value -> use that alias's id
	//    (ambiguous if two ids share the alias)
	// 3. otherwise it is all digits -> accept it as a raw id
	// 4. otherwise -> unresolved
	function resolveVoter(
		raw: string,
		aliasToIds: Map<string, string[]>
	): { ok: true; id: string } | { ok: false; message: string } {
		const val = raw.trim();
		if (val === '') return { ok: false, message: 'a voter field is empty' };
		// hasOwn, not `in`: `in` walks the prototype chain, so a voter typed as
		// "toString" or "constructor" would resolve as a real id and be stored.
		if (Object.hasOwn(aliases, val)) return { ok: true, id: val };
		const ids = aliasToIds.get(val);
		if (ids && ids.length === 1) return { ok: true, id: ids[0] };
		if (ids && ids.length > 1) {
			return { ok: false, message: `"${val}" is ambiguous: matches ids ${ids.join(', ')}` };
		}
		if (/^\d+$/.test(val)) return { ok: true, id: val };
		return { ok: false, message: `could not resolve voter "${val}" to an id or alias` };
	}

	let changesMessage = $state<string>();
	async function sendChanges() {
		changesMessage = undefined;

		blockChanges = true;
		let timeout = 500;
		try {
			// Blank drafts (never filled in) are dropped silently, same as the
			// aliases page's _additions filter.
			const drafts = additions.filter((a) => a.id.trim() !== '' && a.title.trim() !== '');

			if (!(
				Object.keys(titles).length ||
				Object.keys(actives).length ||
				drafts.length ||
				Object.keys(removals).length
			)) {
				changesMessage = 'No content changed.';
				timeout = 0;
				return;
			}

			const seenPollIds = new Set<string>();
			for (const draft of drafts) {
				if (seenPollIds.has(draft.id)) {
					changesMessage = `Duplicate poll id in new polls: ${draft.id}`;
					return;
				}
				seenPollIds.add(draft.id);
				if (pollsById.has(draft.id)) {
					changesMessage = `Poll id ${draft.id} already exists.`;
					return;
				}
			}

			const aliasToIds = buildAliasToIds();
			const resolved: PollAddition[] = [];
			for (const draft of drafts) {
				const seenVoters = new Set<string>();
				const votes: { key: number; voter: string; choice: number }[] = [];
				for (const vote of draft.votes) {
					if (vote.voter.trim() === '') continue;
					if (vote.choice < 0 || vote.choice > 5) {
						changesMessage = `Poll "${draft.title}": choice out of range for voter "${vote.voter}".`;
						return;
					}
					const result = resolveVoter(vote.voter, aliasToIds);
					if (!result.ok) {
						changesMessage = `Poll "${draft.title}": ${result.message}.`;
						return;
					}
					if (seenVoters.has(result.id)) {
						changesMessage = `Poll "${draft.title}": duplicate voter (${vote.voter}).`;
						return;
					}
					seenVoters.add(result.id);
					votes.push({ key: vote.key, voter: result.id, choice: vote.choice });
				}
				resolved.push({ key: draft.key, id: draft.id, title: draft.title, votes });
			}

			const message = await modPolls(titles, actives, resolved, Object.keys(removals));
			if (message) {
				changesMessage = message;
			} else {
				goto('.');
			}
		} finally {
			setTimeout(() => (blockChanges = false), timeout);
		}
	}

	$effect(() => {
		loadContent();
		//
		const tick = setInterval(() => (time = Date.now()), 1000);
		return () => clearInterval(tick);
	});
</script>

<p>
	You can modify polls here: retitle or (de)activate existing polls, remove them, or add new ones.
</p>
<h2>Last updated: {expiryFormatted}</h2>

<ul class="poll-table">
	<li class="poll-row heading">
		<span>id</span>
		<span>date</span>
		<span>total</span>
		<span>active</span>
		<span>topic</span>
		<span>remove</span>
	</li>
	{#each sortedPolls as row (row.id)}
		<li class="poll-row {rowColor(row)}">
			<span class="mono">{row.id}</span>
			<span class="mono">{row.date}</span>
			<span class="mono">{row.total}</span>
			<input
				type="checkbox"
				checked={row.id in actives ? actives[row.id] : row.active}
				disabled={blockChanges}
				onchange={(e) => onActiveChange(row, e.currentTarget.checked)}
			/>
			<input
				value={row.id in titles ? titles[row.id] : row.topic}
				disabled={blockChanges}
				onblur={(e) => onTitleBlur(row, e.currentTarget.value)}
			/>
			<button disabled={blockChanges} onclick={() => onRemoveToggle(row)}>
				{row.id in removals ? '-' : 'X'}
			</button>
		</li>
	{/each}
</ul>

<h2>New Polls</h2>
<ul class="new-polls">
	{#each additions as poll (poll.key)}
		<NewPollForm {poll} disabled={blockChanges} onRemovePoll={() => removeAddition(poll.key)} />
	{/each}
	<button disabled={blockChanges} onclick={addNewPoll}>New Poll</button>
</ul>

<h2>Outgoing changes</h2>
<div class="rowed changes">
	<div class="fill stacked">
		<h2>Retitled</h2>
		<ul>
			{#if Object.keys(titles).length}
				{#each Object.entries(titles) as [id, title]}
					<li>{pollsById.get(id)?.topic} --&gt; {title} ({id})</li>
				{/each}
			{:else}
				<li>No changes yet.</li>
			{/if}
		</ul>
	</div>
	<div class="fill stacked">
		<h2>Active flips</h2>
		<ul>
			{#if Object.keys(actives).length}
				{#each Object.entries(actives) as [id, active]}
					<li>{pollsById.get(id)?.topic} ({id}): {active ? 'active' : 'inactive'}</li>
				{/each}
			{:else}
				<li>No changes yet.</li>
			{/if}
		</ul>
	</div>
	<div class="fill stacked">
		<h2>Added</h2>
		<ul>
			{#if additions.filter((a) => a.id.trim() !== '' && a.title.trim() !== '').length}
				{#each additions.filter((a) => a.id.trim() !== '' && a.title.trim() !== '') as add (add.key)}
					<li>{add.title} ({add.id}) - {add.votes.length} voter(s)</li>
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
					<li>{pollsById.get(id)?.topic} ({id})</li>
				{/each}
			{:else}
				<li>None removed.</li>
			{/if}
		</ul>
	</div>
</div>
<div class="centered stacked">
	<button onclick={sendChanges} disabled={blockChanges}> Send Changes </button>
	{#if changesMessage}
		<p>{changesMessage}</p>
	{/if}
</div>

<style>
	.poll-table {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
		/* the id column is 19-20 digits; let the table scroll rather than
		   squeezing the topic field down to nothing on a narrow viewport. */
		overflow-x: auto;
	}
	/* Every row declares the same tracks, so the header lines up with the
	   cells beneath it even though each row is its own grid container
	   (a shared parent grid would need display:contents, which would drop
	   the row background that carries the red/blue staged-change colour). */
	.poll-row {
		display: grid;
		grid-template-columns: 21ch 11ch 6ch 6ch minmax(14rem, 1fr) 3rem;
		gap: 0.75rem;
		align-items: center;
		padding: 0.3rem 0.5rem;
		border-radius: 3px;
		min-width: fit-content;
	}
	.poll-row.heading {
		font-weight: bold;
		border-bottom: 1px solid currentColor;
		padding-bottom: 0.4rem;
		margin-bottom: 0.2rem;
	}
	.poll-row > span {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.mono {
		font-family: monospace;
		font-variant-numeric: tabular-nums;
		text-align: left;
	}
	.poll-row > input[type='checkbox'] {
		justify-self: center;
	}

	.changes {
		gap: 1.5rem;
		align-items: flex-start;
	}
	.changes h2 {
		font-size: 1rem;
		margin-bottom: 0.25rem;
	}
	.changes ul {
		margin: 0;
		padding-left: 1.2rem;
	}

	.new-polls {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
		align-items: flex-start;
	}

	.red {
		background-color: lightcoral;
	}
	.blue {
		background-color: cornflowerblue;
	}
</style>
