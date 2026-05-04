<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<script lang="ts">
	import { _ } from 'svelte-i18n';
	import { goto } from '$app/navigation';
	import { toast } from 'svelte-sonner';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { auth } from '$lib/stores/auth';
	import { ApiError } from '$lib/api/client';
	import { toastError } from '$lib/toast';

	const APP_VERSION = 'v0.1.0-dev';

	let identifier = $state('');
	let password = $state('');
	let submitting = $state(false);

	let challengeToken = $state<string | null>(null);
	let code = $state('');
	let verifying = $state(false);

	function handleAuthError(err: unknown) {
		if (err instanceof ApiError) {
			if (err.status === 401) {
				toast.error($_('login.error.invalid'), { duration: Infinity });
			} else if (err.status === 429) {
				toast.error($_('login.error.rate_limited'), { duration: Infinity });
			} else {
				toastError(err, $_('login.error.generic'));
			}
		} else {
			toast.error($_('login.error.network'), { duration: Infinity });
		}
	}

	async function onSubmit(e: SubmitEvent) {
		e.preventDefault();
		submitting = true;
		try {
			const result = await auth.login(identifier, password);
			if (result.kind === 'challenge') {
				challengeToken = result.challenge_token;
				code = '';
			} else {
				await goto('/dashboard');
			}
		} catch (err) {
			handleAuthError(err);
		} finally {
			submitting = false;
		}
	}

	async function onVerify(e: SubmitEvent) {
		e.preventDefault();
		if (!challengeToken) return;
		verifying = true;
		try {
			await auth.completeChallenge(challengeToken, code.trim());
			await goto('/dashboard');
		} catch (err) {
			if (err instanceof ApiError && err.status === 401) {
				toast.error($_('login.error.invalid_2fa'), { duration: Infinity });
				code = '';
			} else if (err instanceof ApiError && err.status === 410) {
				challengeToken = null;
				toast.error($_('login.error.challenge_expired'), { duration: Infinity });
			} else {
				handleAuthError(err);
			}
		} finally {
			verifying = false;
		}
	}

	function cancelChallenge() {
		challengeToken = null;
		code = '';
		password = '';
	}
</script>

<div class="flex min-h-screen items-center justify-center bg-background px-4 py-12">
	<div class="w-full max-w-[400px]">
		<div class="mb-8">
			<h1 class="text-2xl font-medium text-foreground">{$_('login.title')}</h1>
		</div>

		{#if challengeToken}
			<form onsubmit={onVerify} class="flex flex-col gap-5">
				<p class="text-sm text-muted-foreground">{$_('login.2fa_help')}</p>
				<div class="flex flex-col gap-2">
					<Label for="code">{$_('login.2fa_code_label')}</Label>
					<Input
						id="code"
						bind:value={code}
						required
						autocomplete="one-time-code"
						inputmode="text"
						autofocus
					/>
				</div>
				<Button type="submit" disabled={verifying} class="mt-2 w-full">
					{verifying ? $_('login.submitting') : $_('login.verify')}
				</Button>
				<Button type="button" variant="outline" onclick={cancelChallenge} class="w-full">
					{$_('common.cancel')}
				</Button>
			</form>
		{:else}
			<form onsubmit={onSubmit} class="flex flex-col gap-5">
				<div class="flex flex-col gap-2">
					<Label for="identifier">{$_('login.identifier_label')}</Label>
					<Input id="identifier" bind:value={identifier} required autocomplete="username" />
				</div>

				<div class="flex flex-col gap-2">
					<Label for="password">{$_('login.password_label')}</Label>
					<Input
						id="password"
						type="password"
						bind:value={password}
						required
						autocomplete="current-password"
					/>
				</div>

				<Button type="submit" disabled={submitting} class="mt-2 w-full">
					{submitting ? $_('login.submitting') : $_('login.submit')}
				</Button>
			</form>
		{/if}

		<p class="mt-8 text-center text-xs text-muted-foreground">
			Cairn · self-hosted · {APP_VERSION}
		</p>
	</div>
</div>
