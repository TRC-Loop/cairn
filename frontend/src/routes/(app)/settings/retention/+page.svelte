<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<script lang="ts">
	import { onMount } from 'svelte';
	import { _ } from 'svelte-i18n';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import {
		getRetentionSettings,
		updateRetentionSettings,
		type RetentionSettings
	} from '$lib/api/retentionSettings';
	import { toastError, toastSuccess } from '$lib/toast';

	let loading = $state(true);
	let saving = $state(false);
	let raw = $state(7);
	let hourly = $state(90);
	let daily = $state(365);
	let keepDailyForever = $state(false);
	let settings = $state<RetentionSettings | null>(null);
	let validationError = $state<string | null>(null);

	function validate(): string | null {
		if (raw < 1 || hourly < 1 || daily < 1) return $_('settings.retention.must_be_positive');
		if (raw > hourly) return $_('settings.retention.raw_le_hourly');
		if (!keepDailyForever && hourly > daily) return $_('settings.retention.hourly_le_daily');
		return null;
	}

	$effect(() => {
		validationError = settings ? validate() : null;
	});

	onMount(async () => {
		try {
			const s = await getRetentionSettings();
			settings = s;
			raw = s.raw_days;
			hourly = s.hourly_days;
			daily = s.daily_days;
			keepDailyForever = s.keep_daily_forever;
		} catch (err) {
			toastError(err);
		} finally {
			loading = false;
		}
	});

	async function save(e: Event) {
		e.preventDefault();
		const err = validate();
		if (err) {
			validationError = err;
			return;
		}
		saving = true;
		try {
			const updated = await updateRetentionSettings({
				raw_days: raw,
				hourly_days: hourly,
				daily_days: daily,
				keep_daily_forever: keepDailyForever
			});
			settings = updated;
			toastSuccess($_('settings.retention.saved'));
		} catch (e2) {
			toastError(e2);
		} finally {
			saving = false;
		}
	}
</script>

{#if loading}
	<p class="text-sm text-muted-foreground">{$_('common.loading')}</p>
{:else}
	<form onsubmit={save} class="space-y-8">
		<section>
			<h2 class="mb-2 text-base font-medium">{$_('settings.retention.section')}</h2>
			<p class="mb-4 text-sm text-muted-foreground">
				{$_('settings.retention.section_help')}
			</p>
			<div class="space-y-4">
				<div class="space-y-1.5">
					<Label for="r-raw">{$_('settings.retention.raw_days')}</Label>
					<Input id="r-raw" type="number" min={1} max={3650} bind:value={raw} required />
					<p class="text-xs text-muted-foreground">{$_('settings.retention.raw_days_help')}</p>
				</div>
				<div class="space-y-1.5">
					<Label for="r-hourly">{$_('settings.retention.hourly_days')}</Label>
					<Input id="r-hourly" type="number" min={1} max={3650} bind:value={hourly} required />
					<p class="text-xs text-muted-foreground">{$_('settings.retention.hourly_days_help')}</p>
				</div>
				<div class="space-y-1.5">
					<Label for="r-daily">{$_('settings.retention.daily_days')}</Label>
					<Input
						id="r-daily"
						type="number"
						min={1}
						max={36500}
						bind:value={daily}
						disabled={keepDailyForever}
						required={!keepDailyForever}
					/>
					<p class="text-xs text-muted-foreground">{$_('settings.retention.daily_days_help')}</p>
				</div>
				<label class="flex items-center gap-2 text-sm">
					<input type="checkbox" bind:checked={keepDailyForever} class="size-4 accent-primary" />
					<span>{$_('settings.retention.keep_forever')}</span>
				</label>
				<p class="text-xs text-muted-foreground">{$_('settings.retention.keep_forever_help')}</p>
			</div>
		</section>

		{#if validationError}
			<p class="text-sm text-destructive">{validationError}</p>
		{/if}

		<div class="flex justify-end border-t border-border pt-6">
			<Button type="submit" disabled={saving || validationError !== null}>
				{saving ? $_('common.saving') : $_('common.save')}
			</Button>
		</div>
	</form>
{/if}
