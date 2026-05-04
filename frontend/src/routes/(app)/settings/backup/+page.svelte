<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<script lang="ts">
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
	import { toastError } from '$lib/toast';

	type Mode = 'db_only' | 'bundle_encrypted' | 'bundle_plain';

	let mode = $state<Mode>('db_only');
	let passphrase = $state('');
	let passphraseConfirm = $state('');
	let plainConfirmed = $state(false);
	let plainModalOpen = $state(false);
	let downloading = $state(false);

	const passphraseTooShort = $derived(
		mode === 'bundle_encrypted' && passphrase.length > 0 && passphrase.length < 12
	);
	const passphraseMismatch = $derived(
		mode === 'bundle_encrypted' &&
			passphraseConfirm.length > 0 &&
			passphrase !== passphraseConfirm
	);

	const formValid = $derived.by(() => {
		if (downloading) return false;
		if (mode === 'db_only') return true;
		if (mode === 'bundle_encrypted') {
			return passphrase.length >= 12 && passphrase === passphraseConfirm;
		}
		// bundle_plain
		return plainConfirmed;
	});

	function readCookie(name: string): string | null {
		if (typeof document === 'undefined') return null;
		const m = document.cookie.match(new RegExp('(^|;\\s*)(' + name + ')=([^;]*)'));
		return m ? decodeURIComponent(m[3]) : null;
	}

	function filenameFromDisposition(h: string | null, fallback: string): string {
		if (!h) return fallback;
		const m = h.match(/filename="?([^";]+)"?/);
		return m ? m[1] : fallback;
	}

	async function startDownload() {
		downloading = true;
		try {
			const csrf = readCookie('cairn_csrf') ?? '';
			const body: Record<string, string> = { mode };
			if (mode === 'bundle_encrypted') body.passphrase = passphrase;
			const res = await fetch('/api/backup/download', {
				method: 'POST',
				credentials: 'same-origin',
				headers: {
					'Content-Type': 'application/json',
					'X-CSRF-Token': csrf
				},
				body: JSON.stringify(body)
			});
			if (!res.ok) {
				let payload: { description?: string } = {};
				try {
					payload = await res.json();
				} catch {
					/* ignore */
				}
				throw new Error(payload.description ?? `HTTP ${res.status}`);
			}
			const blob = await res.blob();
			const fallback =
				mode === 'db_only'
					? 'cairn-backup.db'
					: mode === 'bundle_encrypted'
						? 'cairn-backup.cbackup'
						: 'cairn-backup.tar.gz';
			const filename = filenameFromDisposition(res.headers.get('Content-Disposition'), fallback);
			const url = URL.createObjectURL(blob);
			const a = document.createElement('a');
			a.href = url;
			a.download = filename;
			document.body.appendChild(a);
			a.click();
			a.remove();
			URL.revokeObjectURL(url);
		} catch (err) {
			toastError(err, $_('settings.backup.download_failed'));
		} finally {
			downloading = false;
		}
	}

	function onSubmit(e: Event) {
		e.preventDefault();
		if (downloading) return;
		if (mode === 'bundle_plain') {
			plainModalOpen = true;
			return;
		}
		if (!formValid) return;
		void startDownload();
	}

	function confirmPlain() {
		plainConfirmed = true;
		plainModalOpen = false;
		void startDownload();
	}
</script>

<form onsubmit={onSubmit} class="space-y-10">
	<section>
		<h2 class="mb-2 text-base font-medium">{$_('settings.backup.section_download')}</h2>
		<p class="mb-4 text-sm text-muted-foreground">{$_('settings.backup.subtitle')}</p>

		<fieldset class="space-y-3">
			<legend class="sr-only">{$_('settings.backup.mode_label')}</legend>

			<label class="flex cursor-pointer items-start gap-3 rounded-md border border-border p-3 hover:bg-muted/40">
				<input
					type="radio"
					name="mode"
					value="db_only"
					checked={mode === 'db_only'}
					onchange={() => (mode = 'db_only')}
					class="mt-1"
				/>
				<span class="space-y-1">
					<span class="block text-sm font-medium text-foreground">{$_('settings.backup.mode_db_only')}</span>
					<span class="block text-xs text-muted-foreground">{$_('settings.backup.mode_db_only_help')}</span>
				</span>
			</label>

			<label class="flex cursor-pointer items-start gap-3 rounded-md border border-border p-3 hover:bg-muted/40">
				<input
					type="radio"
					name="mode"
					value="bundle_encrypted"
					checked={mode === 'bundle_encrypted'}
					onchange={() => (mode = 'bundle_encrypted')}
					class="mt-1"
				/>
				<span class="space-y-1">
					<span class="block text-sm font-medium text-foreground">{$_('settings.backup.mode_bundle_encrypted')}</span>
					<span class="block text-xs text-muted-foreground">{$_('settings.backup.mode_bundle_encrypted_help')}</span>
				</span>
			</label>

			<label class="flex cursor-pointer items-start gap-3 rounded-md border border-border p-3 hover:bg-muted/40">
				<input
					type="radio"
					name="mode"
					value="bundle_plain"
					checked={mode === 'bundle_plain'}
					onchange={() => {
						mode = 'bundle_plain';
						plainConfirmed = false;
					}}
					class="mt-1"
				/>
				<span class="space-y-1">
					<span class="block text-sm font-medium text-foreground">{$_('settings.backup.mode_bundle_plain')}</span>
					<span class="block text-xs text-muted-foreground">{$_('settings.backup.mode_bundle_plain_help')}</span>
				</span>
			</label>
		</fieldset>

		{#if mode === 'bundle_encrypted'}
			<div class="mt-5 space-y-4 rounded-md border border-border p-4">
				<div class="space-y-1.5">
					<Label for="bk-pass">{$_('settings.backup.passphrase')}</Label>
					<Input id="bk-pass" type="password" bind:value={passphrase} autocomplete="new-password" />
					<p class="text-xs text-muted-foreground">{$_('settings.backup.passphrase_help')}</p>
					{#if passphraseTooShort}
						<p class="text-xs text-destructive">{$_('settings.backup.passphrase_too_short')}</p>
					{/if}
				</div>
				<div class="space-y-1.5">
					<Label for="bk-pass2">{$_('settings.backup.passphrase_confirm')}</Label>
					<Input
						id="bk-pass2"
						type="password"
						bind:value={passphraseConfirm}
						autocomplete="new-password"
					/>
					{#if passphraseMismatch}
						<p class="text-xs text-destructive">{$_('settings.backup.passphrase_mismatch')}</p>
					{/if}
				</div>
			</div>
		{/if}

		<div class="mt-6 flex justify-end">
			<Button
				type="submit"
				disabled={downloading ||
					(mode === 'bundle_encrypted' && (passphrase.length < 12 || passphrase !== passphraseConfirm))}
			>
				{downloading ? $_('settings.backup.downloading') : $_('settings.backup.download')}
			</Button>
		</div>
	</section>

	<section class="border-t border-border pt-6">
		<h2 class="mb-2 text-base font-medium">{$_('settings.backup.section_restore')}</h2>
		<p class="text-sm text-muted-foreground">{$_('settings.backup.restore_intro')}</p>
		<pre class="mt-3 rounded-md border border-border bg-muted/40 p-3 font-mono text-xs text-foreground"><code>{$_('settings.backup.restore_command_db')}</code></pre>
		<p class="mt-3 text-xs text-muted-foreground">{$_('settings.backup.restore_encrypted_note')}</p>
	</section>
</form>

<Dialog bind:open={plainModalOpen}>
	<DialogContent class="sm:max-w-md">
		<DialogHeader>
			<DialogTitle>{$_('settings.backup.plain_warning_title')}</DialogTitle>
			<DialogDescription>{$_('settings.backup.plain_warning_body')}</DialogDescription>
		</DialogHeader>
		<label class="mt-2 flex items-start gap-2 text-sm text-foreground">
			<input
				type="checkbox"
				bind:checked={plainConfirmed}
				class="mt-1"
			/>
			<span>{$_('settings.backup.plain_warning_checkbox')}</span>
		</label>
		<DialogFooter>
			<Button type="button" variant="ghost" onclick={() => (plainModalOpen = false)}>
				{$_('common.cancel')}
			</Button>
			<Button type="button" disabled={!plainConfirmed} onclick={confirmPlain}>
				{$_('settings.backup.download')}
			</Button>
		</DialogFooter>
	</DialogContent>
</Dialog>
