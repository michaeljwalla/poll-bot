import { StatusCodes } from 'http-status-codes';
import { base } from '$app/paths';

export enum VoteType {
	NA,
	ONE,
	TWO,
	THREE,
	FOUR,
	FIVE
}
export namespace VoteType {
	export function fromString(s: string): VoteType {
		const num = Number(s);
		switch (num) {
			case 1:
			case 2:
			case 3:
			case 4:
			case 5:
				return num as VoteType;
			default:
				return VoteType.NA;
		}
	}
}
export type UserVote = {
	user: string;
	vote: VoteType;
};
export type PollRow = {
	id: string;
	date: string;
	active: boolean;
	topic: string;
	total: number;
};

export async function fetchContent(): Promise<
	| {
			Users: string[];
			Rows: PollRow[];
	  }
	| StatusCodes.UNAUTHORIZED
	| null
> {
	const query = `${base}/api/v1/polls?omitInactive=0&page=`;
	let page = 1;
	const rows: PollRow[] = [];
	const users = new Set<string>();
	while (page > 0) {
		const resp = await fetch(query + page, {
			method: 'GET',
			credentials: 'include'
		});

		if (resp.status == StatusCodes.NO_CONTENT) break;
		else if (resp.status === StatusCodes.UNAUTHORIZED) return resp.status;
		else if (!resp.ok) {
			console.warn('While querying in fetchContent():\n\t', resp.statusText);
			return null;
		}

		let text: string;
		try {
			text = await resp.text();
		} catch (error) {
			console.warn('While querying in fetchContent():\n\t', error);
			return null;
		}

		// Every page carries its own header line, and the voter-column set is
		// computed per page, so page 2's columns may differ from page 1's.
		// Parse each page's header independently; only Users is a running union.
		const lines = text.split('\n').filter((line) => line.trim() !== '');
		if (lines.length > 1) {
			const header = lines[0].split(',').map((s) => s.trim());
			// header: ID, Date, Active, Topic, Total, <voterName...>, <END>
			const voterNames = header.slice(5, -1);
			for (const name of voterNames) {
				if (name) users.add(name);
			}

			for (const line of lines.slice(1)) {
				const cols = line.split(',').map((s) => s.trim());
				// need at least id, date, active, topic, total
				if (cols.length < 5) continue;
				const [id, date, active, topic, total] = cols;
				rows.push({
					id,
					date,
					active: active === '1',
					topic,
					total: Number(total) || 0
				});
			}
		}

		const nextPageHeader = resp.headers.get('X-Next-Page');
		const nextPage = nextPageHeader ? parseInt(nextPageHeader, 10) : 0;
		if (!nextPage || Number.isNaN(nextPage)) break;
		page = nextPage;
	}
	return { Users: Array.from(users), Rows: rows };
}

export async function fetchAliases(): Promise<
	Record<string, string> | StatusCodes.UNAUTHORIZED | null
> {
	const query = `${base}/api/v1/aliases`;

	const resp = await fetch(query, {
		method: 'GET',
		credentials: 'include'
	});
	if (resp.status === StatusCodes.UNAUTHORIZED) return resp.status;
	else if (!resp.ok) {
		console.warn('While querying in fetchAliases():\n\t', resp.statusText);
		return null;
	}
	let data: { id: string; alias: string }[];
	try {
		data = await resp.json();
	} catch (error) {
		console.warn('While querying in fetchAliases():\n\t', error);
		return null;
	}
	let aliases: Record<string, string> = {};
	for (const { id, alias } of data) {
		aliases[id] = alias;
	}
	return aliases;
}

export type Addition = { key: number; id: string; alias: string };
type AliasWeb = { id: string; alias: string };
export async function modAliases(
	changes: Record<string, string[]>,
	additions: Addition[],
	removals: string[]
): Promise<string | null> {
	const query = `${base}/api/v1/aliases`;

	let mods: AliasWeb[] = additions.map((row) => {
		return { id: row.id, alias: row.alias } as AliasWeb;
	});
	for (const [id, alias] of Object.entries(changes)) {
		mods.push({ id: id, alias: alias[1] });
	}
	const changesResp = await fetch(query, {
		method: 'PUT',
		credentials: 'include',
		body: JSON.stringify(mods)
	});
	if (!changesResp.ok) {
		return await changesResp.json();
	}
	const removalsResp = await fetch(query, {
		method: 'DELETE',
		credentials: 'include',
		body: JSON.stringify(removals)
	});
	if (!removalsResp.ok) {
		return await removalsResp.json();
	}
	return null;
}

type ErrResponse = { code: number; error: string; message: string };
async function errMessage(resp: Response): Promise<string> {
	try {
		const data = (await resp.json()) as ErrResponse;
		return data?.message || resp.statusText;
	} catch {
		return resp.statusText;
	}
}

export type PollAddition = {
	key: number;
	id: string;
	title: string;
	votes: { key: number; voter: string; choice: number }[];
};
type VoterWeb = { id: string; choice: number };
type NewPollWeb = { id: string; title: string; votes: VoterWeb[] };
type SetTitleWeb = { id: string; title: string };
type SetActiveWeb = { id: string; active: boolean };

export async function modPolls(
	titles: Record<string, string>,
	actives: Record<string, boolean>,
	additions: PollAddition[],
	removals: string[]
): Promise<string | null> {
	const pollsUrl = `${base}/api/v1/polls`;

	// Order matters: additions (PUT) -> titles (PATCH title) -> actives (PATCH
	// activate) -> removals (DELETE), so a poll added and immediately edited
	// in the same batch behaves, and a removal is not undone by a later write.
	if (additions.length) {
		const body: NewPollWeb[] = additions.map((row) => ({
			id: row.id,
			title: row.title,
			votes: row.votes.map((v) => ({ id: v.voter, choice: v.choice }))
		}));
		const resp = await fetch(pollsUrl, {
			method: 'PUT',
			credentials: 'include',
			body: JSON.stringify(body)
		});
		if (!resp.ok) return await errMessage(resp);
	}

	const titleEntries = Object.entries(titles);
	if (titleEntries.length) {
		const body: SetTitleWeb[] = titleEntries.map(([id, title]) => ({ id, title }));
		const resp = await fetch(pollsUrl + '/title', {
			method: 'PATCH',
			credentials: 'include',
			body: JSON.stringify(body)
		});
		if (!resp.ok) return await errMessage(resp);
	}

	const activeEntries = Object.entries(actives);
	if (activeEntries.length) {
		const body: SetActiveWeb[] = activeEntries.map(([id, active]) => ({ id, active }));
		const resp = await fetch(pollsUrl + '/activate', {
			method: 'PATCH',
			credentials: 'include',
			body: JSON.stringify(body)
		});
		if (!resp.ok) return await errMessage(resp);
	}

	if (removals.length) {
		const resp = await fetch(pollsUrl, {
			method: 'DELETE',
			credentials: 'include',
			body: JSON.stringify(removals)
		});
		if (!resp.ok) return await errMessage(resp);
	}

	return null;
}
