import { writable } from 'svelte/store';

let isAuthenticated = $state<'yes' | 'no' | undefined>(undefined);
let expiresAt = $state(Date.now());

export const getAuthStatus = () => isAuthenticated;
export const getAuthExpiry = () => expiresAt;

// The session cookie is HttpOnly, so the only token the client ever sees is the one
// the root layout load hands down. Call this whenever that value is refreshed.
export function syncAuthExpiry(token?: string) {
	expiresAt = (token && getTokenExpiry(token)) || 0;
}

export function getTokenExpiry(token: string) {
	if (!token) return null;
	try {
		const base64Url = token.split('.')[1];
		const base64 = base64Url.replace(/-/g, '+').replace(/_/g, '/');

		const jsonPayload = decodeURIComponent(
			atob(base64)
				.split('')
				.map((c) => '%' + ('00' + c.charCodeAt(0).toString(16)).slice(-2))
				.join('')
		);

		const { exp } = JSON.parse(jsonPayload);
		return exp ? (exp as number) * 1000 : null;
	} catch (error) {
		return null;
	}
}
type CredResponse = {
	success: boolean;
	message?: string;
};
async function tryCookie(cookie: string): Promise<number | null> {
	const resp = await fetch('/api/v1/login/check', {
		method: 'GET',
		credentials: 'include'
	});
	if (!resp.ok) return null;
	const expiry = getTokenExpiry(cookie);
	if (expiry && Date.now() < expiry) {
		return expiry;
	}
	return null;
}

export async function tryCredentials(
	cookie?: string,
	id?: string,
	pass?: string
): Promise<CredResponse> {
	let success: 'yes' | 'no' = 'no';
	let expiry: number | null = null;

	try {
		if (cookie && (expiry = await tryCookie(cookie))) {
			success = 'yes';
			return { success: true, message: 'valid cookie' };
		}
		if (!(id && pass)) return { success: false };

		const resp = await fetch('/api/v1/login', {
			method: 'POST',
			body: JSON.stringify({
				id: id,
				password: pass
			})
		});
		if (!resp.ok) {
			let message: string | undefined;
			try {
				message = (await resp.json()).message;
			} catch {}
			return { success: false, message };
		}
		//
		success = 'yes';
		return { success: true, message: 'logging in...' };
	} finally {
		isAuthenticated = success;
		if (expiry) expiresAt = expiry;
	}
}
