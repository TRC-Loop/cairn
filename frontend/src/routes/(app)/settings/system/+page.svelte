<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<script lang="ts">
	import { onMount } from 'svelte';
	import { _ } from 'svelte-i18n';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Select from '$lib/components/ui/select';
	import {
		getSystemSettings,
		updateSystemSettings,
		type SystemSettings,
		type ReopenMode
	} from '$lib/api/systemSettings';
	import { toastError, toastSuccess } from '$lib/toast';

	let loading = $state(true);
	let saving = $state(false);
	let format = $state('INC-{id}');
	let reopenWindow = $state(0);
	let reopenMode = $state<string>('flapping_only');
	let settings = $state<SystemSettings | null>(null);

	function pad(n: number, w: number) {
		return String(n).padStart(w, '0');
	}
	const now = new Date();
	const datetimeSample = `${now.getUTCFullYear()}${pad(now.getUTCMonth() + 1, 2)}${pad(now.getUTCDate(), 2)}T${pad(now.getUTCHours(), 2)}${pad(now.getUTCMinutes(), 2)}${pad(now.getUTCSeconds(), 2)}`;
	const yearSample = String(now.getUTCFullYear());
	const monthSample = pad(now.getUTCMonth() + 1, 2);
	const daySample = pad(now.getUTCDate(), 2);

	const preview = $derived(format.replace('{id}', '0042'));
	const hasIdToken = $derived(format.includes('{id}'));

	onMount(async () => {
		try {
			const s = await getSystemSettings();
			settings = s;
			format = s.incident_id_format;
			reopenWindow = s.incident_reopen_window_seconds;
			reopenMode = s.incident_reopen_mode;
		} catch (err) {
			toastError(err);
		} finally {
			loading = false;
		}
	});

	async function save(e: Event) {
		e.preventDefault();
		if (!hasIdToken) return;
		saving = true;
		try {
			const updated = await updateSystemSettings({
				incident_id_format: format,
				incident_reopen_window_seconds: reopenWindow,
				incident_reopen_mode: reopenMode as ReopenMode
			});
			settings = updated;
			toastSuccess($_('settings.system.saved'));
		} catch (err) {
			toastError(err);
		} finally {
			saving = false;
		}
	}

	function reopenLabel(m: ReopenMode) {
		if (m === 'always') return $_('settings.system.reopen_mode_always');
		if (m === 'never') return $_('settings.system.reopen_mode_never');
		return $_('settings.system.reopen_mode_flapping');
	}
</script>

{#if loading}
	<p class="text-sm text-muted-foreground">{$_('common.loading')}</p>
{:else}
	<form onsubmit={save} class="space-y-10">
		<section>
			<h2 class="mb-2 text-base font-medium">{$_('settings.system.section_incidents')}</h2>
			<p class="mb-4 text-sm text-muted-foreground">
				{$_('settings.system.section_incidents_help')}
			</p>
			<div class="space-y-4">
				<div class="space-y-1.5">
					<div class="flex items-center gap-2">
						<Label for="s-fmt">{$_('settings.system.id_format')}</Label>
						<details class="text-xs">
							<summary class="cursor-pointer text-muted-foreground hover:text-foreground" aria-label={$_('settings.system.id_format_placeholders')}>
								(?)
							</summary>
							<div class="mt-2 rounded-md border bg-muted/30 p-3">
								<p class="mb-2 text-xs text-muted-foreground">
									{$_('settings.system.id_format_placeholders')}
								</p>
								<table class="text-xs">
									<tbody>
										<tr><td class="pr-3 font-mono">{'{id}'}</td><td class="font-mono">42</td></tr>
										<tr><td class="pr-3 font-mono">{'{year}'}</td><td class="font-mono">{yearSample}</td></tr>
										<tr><td class="pr-3 font-mono">{'{month}'}</td><td class="font-mono">{monthSample}</td></tr>
										<tr><td class="pr-3 font-mono">{'{day}'}</td><td class="font-mono">{daySample}</td></tr>
										<tr><td class="pr-3 font-mono">{'{datetime}'}</td><td class="font-mono">{datetimeSample}</td></tr>
									</tbody>
								</table>
							</div>
						</details>
					</div>
					<Input id="s-fmt" bind:value={format} maxlength={64} required />
					<p class="text-xs text-muted-foreground">
						{$_('settings.system.id_format_help')}
					</p>
					<p class="text-xs">
						<span class="text-muted-foreground">{$_('settings.system.id_format_preview')}: </span>
						<span class="font-mono text-foreground">{preview}</span>
					</p>
					{#if !hasIdToken}
						<p class="text-xs text-destructive">{$_('settings.system.id_format_missing_token')}</p>
					{/if}
				</div>
			</div>
		</section>

		<section>
			<h2 class="mb-2 text-base font-medium">{$_('settings.system.section_reopen')}</h2>
			<p class="mb-4 text-sm text-muted-foreground">
				{$_('settings.system.section_reopen_help')}
			</p>
			<div class="space-y-4">
				<div class="space-y-1.5">
					<Label for="s-rw">{$_('settings.system.reopen_window')}</Label>
					<Input
						id="s-rw"
						type="number"
						min={0}
						max={86400}
						step={1}
						bind:value={reopenWindow}
						required
					/>
					<p class="text-xs text-muted-foreground">
						{$_('settings.system.reopen_window_help')}
					</p>
				</div>
				<div class="space-y-1.5">
					<Label for="s-rm">{$_('settings.system.reopen_mode')}</Label>
					<Select.Root type="single" bind:value={reopenMode}>
						<Select.Trigger id="s-rm" class="w-full">
							{reopenLabel(reopenMode as ReopenMode)}
						</Select.Trigger>
						<Select.Content>
							<Select.Item value="flapping_only">
								{$_('settings.system.reopen_mode_flapping')}
							</Select.Item>
							<Select.Item value="always">
								{$_('settings.system.reopen_mode_always')}
							</Select.Item>
							<Select.Item value="never">
								{$_('settings.system.reopen_mode_never')}
							</Select.Item>
						</Select.Content>
					</Select.Root>
					<div class="space-y-1 rounded-md border bg-muted/30 p-3 text-xs text-muted-foreground">
						<p>
							<span class="font-medium text-foreground">{$_('settings.system.reopen_mode_always')}:</span>
							{$_('settings.system.reopen_mode_always_help')}
						</p>
						<p>
							<span class="font-medium text-foreground">{$_('settings.system.reopen_mode_never')}:</span>
							{$_('settings.system.reopen_mode_never_help')}
						</p>
						<p>
							<span class="font-medium text-foreground">{$_('settings.system.reopen_mode_flapping')}:</span>
							{$_('settings.system.reopen_mode_flapping_help')}
						</p>
					</div>
				</div>
			</div>
		</section>

		<div class="flex justify-end border-t border-border pt-6">
			<Button type="submit" disabled={saving || !hasIdToken}>
				{saving ? $_('common.saving') : $_('common.save')}
			</Button>
		</div>
	</form>
{/if}
