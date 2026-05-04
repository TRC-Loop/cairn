<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<script lang="ts">
	import { onMount } from 'svelte';
	import { _, locale } from 'svelte-i18n';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Select from '$lib/components/ui/select';
	import { auth } from '$lib/stores/auth';
	import { updateUser, changePassword } from '$lib/api/users';
	import { toastError, toastSuccess } from '$lib/toast';
	import TwoFactorSection from '$lib/components/auth/TwoFactorSection.svelte';

	const user = $derived($auth.user);

	let displayName = $state('');
	let email = $state('');
	let savingProfile = $state(false);

	let currentPassword = $state('');
	let newPassword = $state('');
	let confirmPassword = $state('');
	let savingPassword = $state(false);
	let passwordError = $state<string | null>(null);

	let language = $state<string>('auto');

	onMount(() => {
		if (user) {
			displayName = user.display_name;
			email = user.email;
		}
		try {
			const stored = localStorage.getItem('cairn_language');
			if (stored === 'en' || stored === 'de') language = stored;
		} catch {
			/* ignore */
		}
	});

	async function saveProfile(e: Event) {
		e.preventDefault();
		if (!user) return;
		savingProfile = true;
		try {
			const updated = await updateUser(user.id, { display_name: displayName, email });
			await auth.init();
			toastSuccess($_('settings.profile.saved'));
			displayName = updated.display_name;
			email = updated.email;
		} catch (err) {
			toastError(err);
		} finally {
			savingProfile = false;
		}
	}

	async function savePassword(e: Event) {
		e.preventDefault();
		if (!user) return;
		passwordError = null;
		if (newPassword.length < 12) {
			passwordError = $_('settings.profile.password_too_short');
			return;
		}
		if (newPassword !== confirmPassword) {
			passwordError = $_('settings.profile.password_mismatch');
			return;
		}
		savingPassword = true;
		try {
			await changePassword(user.id, {
				current_password: currentPassword,
				new_password: newPassword
			});
			currentPassword = '';
			newPassword = '';
			confirmPassword = '';
			toastSuccess($_('settings.profile.password_changed'));
		} catch (err) {
			toastError(err);
		} finally {
			savingPassword = false;
		}
	}

	function saveLanguage() {
		try {
			if (language === 'auto') {
				localStorage.removeItem('cairn_language');
			} else {
				localStorage.setItem('cairn_language', language);
			}
		} catch {
			/* ignore */
		}
		if (language !== 'auto') {
			locale.set(language);
		}
		toastSuccess($_('settings.profile.language_saved'));
	}
</script>

{#if user}
	<div class="space-y-10">
		<section>
			<h2 class="mb-2 text-base font-medium">{$_('settings.profile.section_info')}</h2>
			<p class="mb-4 text-sm text-muted-foreground">
				{$_('settings.profile.section_info_help')}
			</p>
			<form onsubmit={saveProfile} class="space-y-4">
				<div class="space-y-1.5">
					<Label for="p-username">{$_('settings.profile.username')}</Label>
					<Input id="p-username" value={user.username} readonly disabled />
					<p class="text-xs text-muted-foreground">
						{$_('settings.profile.username_immutable')}
					</p>
				</div>
				<div class="space-y-1.5">
					<Label for="p-display">{$_('settings.profile.display_name')}</Label>
					<Input id="p-display" bind:value={displayName} maxlength={100} required />
				</div>
				<div class="space-y-1.5">
					<Label for="p-email">{$_('settings.profile.email')}</Label>
					<Input id="p-email" type="email" bind:value={email} required />
				</div>
				<div class="flex justify-end">
					<Button type="submit" disabled={savingProfile}>
						{savingProfile ? $_('common.saving') : $_('common.save')}
					</Button>
				</div>
			</form>
		</section>

		<section>
			<h2 class="mb-2 text-base font-medium">{$_('settings.profile.section_password')}</h2>
			<p class="mb-4 text-sm text-muted-foreground">
				{$_('settings.profile.section_password_help')}
			</p>
			<form onsubmit={savePassword} class="space-y-4">
				<div class="space-y-1.5">
					<Label for="p-cur">{$_('settings.profile.current_password')}</Label>
					<Input id="p-cur" type="password" bind:value={currentPassword} required />
				</div>
				<div class="space-y-1.5">
					<Label for="p-new">{$_('settings.profile.new_password')}</Label>
					<Input id="p-new" type="password" bind:value={newPassword} minlength={12} required />
				</div>
				<div class="space-y-1.5">
					<Label for="p-conf">{$_('settings.profile.confirm_password')}</Label>
					<Input id="p-conf" type="password" bind:value={confirmPassword} required />
				</div>
				{#if passwordError}
					<p class="text-sm text-destructive">{passwordError}</p>
				{/if}
				<div class="flex justify-end">
					<Button type="submit" disabled={savingPassword}>
						{savingPassword ? $_('common.saving') : $_('settings.profile.change_password')}
					</Button>
				</div>
			</form>
		</section>

		<TwoFactorSection />

		<section>
			<h2 class="mb-2 text-base font-medium">{$_('settings.profile.section_prefs')}</h2>
			<form
				onsubmit={(e) => {
					e.preventDefault();
					saveLanguage();
				}}
				class="space-y-4"
			>
				<div class="space-y-1.5">
					<Label for="p-lang">{$_('settings.profile.language')}</Label>
					<Select.Root type="single" bind:value={language}>
						<Select.Trigger id="p-lang" class="w-full">
							{language === 'auto'
								? $_('settings.profile.language_auto')
								: language === 'en'
									? 'English'
									: 'Deutsch'}
						</Select.Trigger>
						<Select.Content>
							<Select.Item value="auto">{$_('settings.profile.language_auto')}</Select.Item>
							<Select.Item value="en">English</Select.Item>
							<Select.Item value="de">Deutsch</Select.Item>
						</Select.Content>
					</Select.Root>
				</div>
				<div class="flex justify-end">
					<Button type="submit">{$_('common.save')}</Button>
				</div>
			</form>
		</section>
	</div>
{/if}
