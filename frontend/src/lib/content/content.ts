import { StatusCodes } from 'http-status-codes';

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
	const query = '/api/v1/polls?omitInactive=1&page=';
	let page = 1;
	let rows = [];
	while (page > 0) {
		const resp = await fetch(query + page, {
			method: 'GET',
			credentials: 'include'
		});

		if (resp.status == StatusCodes.NO_CONTENT) break;
		else if (resp.status === StatusCodes.UNAUTHORIZED) return resp.status;
		else if (!resp.ok) {
			console.warn('While querying in fetchAliases():\n\t', resp.statusText);
			return null;
		}
		page++;
		// check content not nothing
	}
	return null;
}

export async function fetchAliases(): Promise<
	Record<string, string> | StatusCodes.UNAUTHORIZED | null
> {
	const query = '/api/v1/aliases';

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
	const query = '/api/v1/aliases';

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
