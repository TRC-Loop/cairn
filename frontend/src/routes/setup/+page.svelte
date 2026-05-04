<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<script lang="ts">
	import { _ } from 'svelte-i18n';
	import { goto } from '$app/navigation';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { auth } from '$lib/stores/auth';
	import { ApiError } from '$lib/api/client';
	import { toastError } from '$lib/toast';
	import { fieldErrorText } from '$lib/validate';

	let displayName = $state('');
	let username = $state('');
	let email = $state('');
	let password = $state('');
	let passwordConfirm = $state('');

	let submitting = $state(false);
	let fieldErrors = $state<Record<string, string>>({});

	function clientValidate(): Record<string, string> {
		const errs: Record<string, string> = {};
		if (password.length > 0 && password.length < 12) {
			errs.password = $_('setup.error.password_short');
		}
		if (password && passwordConfirm && password !== passwordConfirm) {
			errs.password_confirm = $_('setup.error.password_mismatch');
		}
		return errs;
	}

	async function onSubmit(e: SubmitEvent) {
		e.preventDefault();
		const live = clientValidate();
		if (Object.keys(live).length > 0) {
			fieldErrors = live;
			return;
		}
		submitting = true;
		fieldErrors = {};
		try {
			await auth.setupComplete({
				username,
				email,
				display_name: displayName,
				password
			});
			await goto('/dashboard');
		} catch (err) {
			if (err instanceof ApiError && err.fields) fieldErrors = err.fields;
			toastError(err, $_('setup.error.generic'));
		} finally {
			submitting = false;
		}
	}
</script>

<div class="flex min-h-screen items-center justify-center bg-background px-4 py-12">
	<div class="w-full max-w-[480px]">
		<div class="mb-8">
			<h1 class="text-2xl font-medium text-foreground">{$_('setup.title')}</h1>
			<p class="mt-2 text-sm text-muted-foreground">{$_('setup.subtitle')}</p>
		</div>

		<form onsubmit={onSubmit} class="flex flex-col gap-5">
			<div class="flex flex-col gap-2">
				<Label for="display_name">{$_('setup.display_name_label')}</Label>
				<Input id="display_name" bind:value={displayName} required autocomplete="name" />
				{#if fieldErrors.display_name}
					<p class="text-xs text-destructive">{fieldErrorText(fieldErrors.display_name)}</p>
				{/if}
			</div>

			<div class="flex flex-col gap-2">
				<Label for="username">{$_('setup.username_label')}</Label>
				<Input id="username" bind:value={username} required autocomplete="username" />
				<p class="text-xs text-muted-foreground">{$_('setup.username_help')}</p>
				{#if fieldErrors.username}
					<p class="text-xs text-destructive">{fieldErrorText(fieldErrors.username)}</p>
				{/if}
			</div>

			<div class="flex flex-col gap-2">
				<Label for="email">{$_('setup.email_label')}</Label>
				<Input id="email" type="email" bind:value={email} required autocomplete="email" />
				{#if fieldErrors.email}
					<p class="text-xs text-destructive">{fieldErrorText(fieldErrors.email)}</p>
				{/if}
			</div>

			<div class="flex flex-col gap-2">
				<Label for="password">{$_('setup.password_label')}</Label>
				<Input
					id="password"
					type="password"
					bind:value={password}
					required
					autocomplete="new-password"
				/>
				<p class="text-xs text-muted-foreground">{$_('setup.password_help')}</p>
				{#if fieldErrors.password}
					<p class="text-xs text-destructive">{fieldErrorText(fieldErrors.password)}</p>
				{/if}
			</div>

			<div class="flex flex-col gap-2">
				<Label for="password_confirm">{$_('setup.password_confirm_label')}</Label>
				<Input
					id="password_confirm"
					type="password"
					bind:value={passwordConfirm}
					required
					autocomplete="new-password"
				/>
				{#if fieldErrors.password_confirm}
					<p class="text-xs text-destructive">{fieldErrorText(fieldErrors.password_confirm)}</p>
				{/if}
			</div>

			<Button type="submit" disabled={submitting} class="mt-2 w-full">
				{submitting ? $_('setup.submitting') : $_('setup.submit')}
			</Button>
		</form>

		<p class="mt-6 text-center text-xs text-muted-foreground">
			{$_('setup.help_prefix')}
			<a href="/docs" class="underline hover:text-foreground">{$_('setup.help_link')}</a>
		</p>
	</div>
</div>
