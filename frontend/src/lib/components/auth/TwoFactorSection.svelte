<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<script lang="ts">
	import { onMount } from 'svelte';
	import { _ } from 'svelte-i18n';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import {
		Dialog,
		DialogContent,
		DialogHeader,
		DialogTitle,
		DialogDescription,
		DialogFooter
	} from '$lib/components/ui/dialog';
	import {
		getTwoFAStatus,
		startTwoFASetup,
		confirmTwoFASetup,
		disableTwoFA,
		regenerateRecoveryCodes,
		type TwoFAStatus,
		type TwoFASetupResponse
	} from '$lib/api/twofa';
	import { auth } from '$lib/stores/auth';
	import { toastError, toastSuccess } from '$lib/toast';

	let status = $state<TwoFAStatus | null>(null);
	let loading = $state(true);

	let enrollOpen = $state(false);
	let enrollStep = $state<'scan' | 'codes'>('scan');
	let enrollData = $state<TwoFASetupResponse | null>(null);
	let enrollCode = $state('');
	let enrollSaving = $state(false);
	let recoveryCodes = $state<string[]>([]);
	let acknowledged = $state(false);

	let disableOpen = $state(false);
	let disablePassword = $state('');
	let disableCode = $state('');
	let disableSaving = $state(false);

	let regenOpen = $state(false);
	let regenPassword = $state('');
	let regenCode = $state('');
	let regenSaving = $state(false);
	let regenStep = $state<'form' | 'codes'>('form');

	async function refresh() {
		loading = true;
		try {
			status = await getTwoFAStatus();
		} catch (err) {
			toastError(err);
		} finally {
			loading = false;
		}
	}

	onMount(refresh);

	async function startEnrollment() {
		try {
			enrollData = await startTwoFASetup();
			enrollStep = 'scan';
			enrollCode = '';
			recoveryCodes = [];
			acknowledged = false;
			enrollOpen = true;
		} catch (err) {
			toastError(err);
		}
	}

	async function submitEnrollConfirm(e: Event) {
		e.preventDefault();
		enrollSaving = true;
		try {
			const resp = await confirmTwoFASetup(enrollCode.trim());
			recoveryCodes = resp.recovery_codes;
			enrollStep = 'codes';
			await auth.init();
		} catch (err) {
			toastError(err);
		} finally {
			enrollSaving = false;
		}
	}

	async function finishEnrollment() {
		enrollOpen = false;
		await refresh();
		toastSuccess($_('auth.2fa.enabled_success'));
	}

	function openDisable() {
		disablePassword = '';
		disableCode = '';
		disableOpen = true;
	}

	async function submitDisable(e: Event) {
		e.preventDefault();
		disableSaving = true;
		try {
			await disableTwoFA(disablePassword, disableCode.trim());
			disableOpen = false;
			await auth.init();
			await refresh();
			toastSuccess($_('auth.2fa.disabled_success'));
		} catch (err) {
			toastError(err);
		} finally {
			disableSaving = false;
		}
	}

	function openRegen() {
		regenPassword = '';
		regenCode = '';
		recoveryCodes = [];
		acknowledged = false;
		regenStep = 'form';
		regenOpen = true;
	}

	async function submitRegen(e: Event) {
		e.preventDefault();
		regenSaving = true;
		try {
			const resp = await regenerateRecoveryCodes(regenPassword, regenCode.trim());
			recoveryCodes = resp.recovery_codes;
			regenStep = 'codes';
		} catch (err) {
			toastError(err);
		} finally {
			regenSaving = false;
		}
	}

	async function finishRegen() {
		regenOpen = false;
		await refresh();
		toastSuccess($_('auth.2fa.codes_regenerated'));
	}

	async function copyCodes() {
		try {
			await navigator.clipboard.writeText(recoveryCodes.join('\n'));
			toastSuccess($_('auth.2fa.copied'));
		} catch {
			/* ignore */
		}
	}

	function downloadCodes() {
		const blob = new Blob([recoveryCodes.join('\n') + '\n'], { type: 'text/plain' });
		const url = URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		a.download = 'cairn-recovery-codes.txt';
		document.body.appendChild(a);
		a.click();
		document.body.removeChild(a);
		URL.revokeObjectURL(url);
	}

	function formatEnrolledAt(iso: string | null): string {
		if (!iso) return '';
		try {
			return new Date(iso).toLocaleDateString();
		} catch {
			return iso;
		}
	}
</script>

<section>
	<h2 class="mb-2 text-base font-medium">{$_('auth.2fa.section_title')}</h2>
	{#if loading || !status}
		<p class="text-sm text-muted-foreground">{$_('common.loading')}</p>
	{:else if status.enabled}
		<p class="mb-4 text-sm text-muted-foreground">
			{$_('auth.2fa.enabled_summary', {
				values: {
					date: formatEnrolledAt(status.enrolled_at),
					n: status.recovery_codes_remaining
				}
			})}
		</p>
		<div class="flex flex-wrap gap-2">
			<Button variant="outline" onclick={openDisable}>{$_('auth.2fa.disable')}</Button>
			<Button variant="outline" onclick={openRegen}>{$_('auth.2fa.regenerate')}</Button>
		</div>
	{:else}
		<p class="mb-4 text-sm text-muted-foreground">
			{$_('auth.2fa.disabled_help')}
		</p>
		<Button onclick={startEnrollment}>{$_('auth.2fa.enable')}</Button>
	{/if}
</section>

<Dialog bind:open={enrollOpen}>
	<DialogContent class="max-w-md">
		<DialogHeader>
			<DialogTitle>{$_('auth.2fa.enroll_title')}</DialogTitle>
		</DialogHeader>
		{#if enrollStep === 'scan' && enrollData}
			<form onsubmit={submitEnrollConfirm} class="space-y-4">
				<p class="text-sm text-muted-foreground">{$_('auth.2fa.enroll_help')}</p>
				<div class="flex justify-center">
					<img
						src={enrollData.qr_code_data_url}
						alt="TOTP QR code"
						width="220"
						height="220"
						class="rounded border border-border bg-white p-2"
					/>
				</div>
				<div class="space-y-1.5">
					<Label for="2fa-secret">{$_('auth.2fa.setup_key')}</Label>
					<Input id="2fa-secret" value={enrollData.secret} readonly />
				</div>
				<div class="space-y-1.5">
					<Label for="2fa-code">{$_('auth.2fa.enter_code')}</Label>
					<Input
						id="2fa-code"
						bind:value={enrollCode}
						required
						inputmode="numeric"
						pattern="[0-9]{'{6}'}"
						maxlength={6}
						autocomplete="one-time-code"
					/>
				</div>
				<DialogFooter>
					<Button type="button" variant="outline" onclick={() => (enrollOpen = false)}>
						{$_('common.cancel')}
					</Button>
					<Button type="submit" disabled={enrollSaving}>
						{enrollSaving ? $_('common.saving') : $_('auth.2fa.confirm')}
					</Button>
				</DialogFooter>
			</form>
		{:else if enrollStep === 'codes'}
			<div class="space-y-4">
				<DialogDescription>{$_('auth.2fa.codes_save_now')}</DialogDescription>
				<div
					class="grid grid-cols-2 gap-x-6 gap-y-1 rounded border border-border bg-muted/30 p-3 font-mono text-sm"
				>
					{#each recoveryCodes as c (c)}
						<span>{c}</span>
					{/each}
				</div>
				<div class="flex flex-wrap gap-2">
					<Button variant="outline" onclick={copyCodes}>{$_('auth.2fa.copy')}</Button>
					<Button variant="outline" onclick={downloadCodes}>{$_('auth.2fa.download')}</Button>
				</div>
				<label class="flex items-center gap-2 text-sm">
					<input type="checkbox" bind:checked={acknowledged} class="size-4" />
					{$_('auth.2fa.codes_acknowledge')}
				</label>
				<DialogFooter>
					<Button onclick={finishEnrollment} disabled={!acknowledged}>{$_('common.done')}</Button>
				</DialogFooter>
			</div>
		{/if}
	</DialogContent>
</Dialog>

<Dialog bind:open={disableOpen}>
	<DialogContent>
		<DialogHeader>
			<DialogTitle>{$_('auth.2fa.disable_title')}</DialogTitle>
			<DialogDescription>{$_('auth.2fa.disable_help')}</DialogDescription>
		</DialogHeader>
		<form onsubmit={submitDisable} class="space-y-3">
			<div class="space-y-1.5">
				<Label for="dis-pw">{$_('auth.2fa.current_password')}</Label>
				<Input id="dis-pw" type="password" bind:value={disablePassword} required />
			</div>
			<div class="space-y-1.5">
				<Label for="dis-code">{$_('auth.2fa.code_or_recovery')}</Label>
				<Input id="dis-code" bind:value={disableCode} required autocomplete="one-time-code" />
			</div>
			<DialogFooter>
				<Button type="button" variant="outline" onclick={() => (disableOpen = false)}>
					{$_('common.cancel')}
				</Button>
				<Button type="submit" variant="destructive" disabled={disableSaving}>
					{disableSaving ? $_('common.saving') : $_('auth.2fa.disable')}
				</Button>
			</DialogFooter>
		</form>
	</DialogContent>
</Dialog>

<Dialog bind:open={regenOpen}>
	<DialogContent class="max-w-md">
		<DialogHeader>
			<DialogTitle>{$_('auth.2fa.regenerate_title')}</DialogTitle>
		</DialogHeader>
		{#if regenStep === 'form'}
			<form onsubmit={submitRegen} class="space-y-3">
				<DialogDescription>{$_('auth.2fa.regenerate_help')}</DialogDescription>
				<div class="space-y-1.5">
					<Label for="rg-pw">{$_('auth.2fa.current_password')}</Label>
					<Input id="rg-pw" type="password" bind:value={regenPassword} required />
				</div>
				<div class="space-y-1.5">
					<Label for="rg-code">{$_('auth.2fa.code_or_recovery')}</Label>
					<Input id="rg-code" bind:value={regenCode} required autocomplete="one-time-code" />
				</div>
				<DialogFooter>
					<Button type="button" variant="outline" onclick={() => (regenOpen = false)}>
						{$_('common.cancel')}
					</Button>
					<Button type="submit" disabled={regenSaving}>
						{regenSaving ? $_('common.saving') : $_('auth.2fa.regenerate')}
					</Button>
				</DialogFooter>
			</form>
		{:else}
			<div class="space-y-4">
				<DialogDescription>{$_('auth.2fa.codes_save_now')}</DialogDescription>
				<div
					class="grid grid-cols-2 gap-x-6 gap-y-1 rounded border border-border bg-muted/30 p-3 font-mono text-sm"
				>
					{#each recoveryCodes as c (c)}
						<span>{c}</span>
					{/each}
				</div>
				<div class="flex flex-wrap gap-2">
					<Button variant="outline" onclick={copyCodes}>{$_('auth.2fa.copy')}</Button>
					<Button variant="outline" onclick={downloadCodes}>{$_('auth.2fa.download')}</Button>
				</div>
				<label class="flex items-center gap-2 text-sm">
					<input type="checkbox" bind:checked={acknowledged} class="size-4" />
					{$_('auth.2fa.codes_acknowledge')}
				</label>
				<DialogFooter>
					<Button onclick={finishRegen} disabled={!acknowledged}>{$_('common.done')}</Button>
				</DialogFooter>
			</div>
		{/if}
	</DialogContent>
</Dialog>
