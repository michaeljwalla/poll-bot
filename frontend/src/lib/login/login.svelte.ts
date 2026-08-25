import { base } from '$app/paths';

let isAuthenticated = $state<'yes' | 'no' | undefined>(undefined);
let expiresAt = $state(0);

export const getAuthStatus = () => isAuthenticated;
export const getAuthExpiry = () => expiresAt;

type CheckResponse = { expiresAt?: number };

// The session cookie is HttpOnly, so the client can never read the token or its
// claims. /login/check is the only source of truth for both "am I still logged
// in" and "for how long" — it returns the expiry as unix milliseconds.
export async function checkSession(): Promise<number | null> {
	let resp: Response;
	try {
		resp = await fetch(`${base}/api/v1/login/check`, {
			method: 'GET',
			credentials: 'include'
		});
	} catch {
		return null;
	}
	if (!resp.ok) return null;
	try {
		const { expiresAt: exp }: CheckResponse = await resp.json();
		return typeof exp === 'number' && exp > Date.now() ? exp : null;
	} catch {
		return null;
	}
}

type CredResponse = {
	success: boolean;
	message?: string;
};

// With no id/pass this only revalidates an existing cookie; with them it logs
// in first. Either way the expiry is refreshed from the server on success.
export async function tryCredentials(id?: string, pass?: string): Promise<CredResponse> {
	let success: 'yes' | 'no' = 'no';
	let expiry: number | null = null;

	try {
		if ((expiry = await checkSession())) {
			success = 'yes';
			return { success: true, message: 'valid session' };
		}
		if (!(id && pass)) return { success: false };

		const resp = await fetch(`${base}/api/v1/login`, {
			method: 'POST',
			credentials: 'include',
			body: JSON.stringify({ id: id, password: pass })
		});
		if (!resp.ok) {
			let message: string | undefined;
			try {
				message = (await resp.json()).message;
			} catch {}
			return { success: false, message };
		}

		//the login response set the cookie; ask what it is worth
		expiry = await checkSession();
		success = 'yes';
		return { success: true, message: 'logging in...' };
	} finally {
		isAuthenticated = success;
		if (expiry) expiresAt = expiry;
	}
}
