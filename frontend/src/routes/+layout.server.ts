export function load({ cookies }) {
	const auth = cookies.get('Authorization');
	return {
		auth: auth
	};
}
