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
					<Label for="s-fmt">{$_('settings.system.id_format')}</Label>
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
					<p class="text-xs text-muted-foreground">
						{reopenMode === 'always'
							? $_('settings.system.reopen_mode_always_help')
							: reopenMode === 'never'
								? $_('settings.system.reopen_mode_never_help')
								: $_('settings.system.reopen_mode_flapping_help')}
					</p>
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
