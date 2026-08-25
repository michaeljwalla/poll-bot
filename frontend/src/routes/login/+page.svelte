<script lang="ts">
	import { goto, invalidateAll } from '$app/navigation';
	import { tryCredentials } from '$lib/login/login.svelte';

	let idField = $state('');
	let passField = $state('');

	let submitDisabled = $state(false);
	let submitError = $state<string | undefined>(undefined);

	const onSubmit = async () => {
		submitDisabled = true;
		submitError = undefined;
		const authenticated = await tryCredentials(undefined, idField, passField);
		if (authenticated.success) {
			// pick up the freshly-set Authorization cookie so the expiry is known
			await invalidateAll();
			goto('/home');
		} else {
			submitDisabled = false;
			submitError = 'Failure: ' + (authenticated.message ?? 'No response received.');
		}
	};
</script>

<div class="centered stacked">
	<h1>Login</h1>
	<div class="stacked">
		<input bind:value={idField} disabled={submitDisabled} placeholder="Enter your user id" />
		<input
			bind:value={passField}
			type="password"
			disabled={submitDisabled}
			placeholder="Enter your password"
		/>
		<button disabled={submitDisabled} onclick={onSubmit}
			>{submitDisabled ? 'Submitting...' : 'Submit'}</button
		>
	</div>
	{#if submitError}
		<p>{submitError}</p>
	{/if}
</div>
