<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<script lang="ts">
	import { onMount } from 'svelte';
	import { _ } from 'svelte-i18n';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Select from '$lib/components/ui/select';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Alert, AlertTitle, AlertDescription } from '$lib/components/ui/alert';
	import PageHeader from '$lib/components/common/PageHeader.svelte';
	import {
		getIncident,
		patchIncident,
		type IncidentSeverity
	} from '$lib/api/incidents';
	import { toastError, toastSuccess } from '$lib/toast';

	const incidentId = $derived(Number(page.params.id));

	let title = $state('');
	let severity = $state<string>('minor');
	let loading = $state(true);
	let notFound = $state(false);
	let saving = $state(false);

	onMount(async () => {
		try {
			const detail = await getIncident(incidentId);
			title = detail.incident.title;
			severity = detail.incident.severity;
		} catch (err) {
			const e = err as { status?: number };
			if (e?.status === 404) notFound = true;
			else toastError(err, $_('common.error_generic'));
		} finally {
			loading = false;
		}
	});

	async function submit(e: Event) {
		e.preventDefault();
		const t = title.trim();
		if (!t) {
			toastError(null, $_('common.required'));
			return;
		}
		saving = true;
		try {
			await patchIncident(incidentId, {
				title: t,
				severity: severity as IncidentSeverity
			});
			toastSuccess($_('incidents.edit.success'));
			void goto(`/incidents/${incidentId}`);
		} catch (err) {
			toastError(err, $_('common.error_generic'));
		} finally {
			saving = false;
		}
	}
</script>

<div class="p-6">
	<a
		href={`/incidents/${incidentId}`}
		class="mb-4 inline-block text-sm text-muted-foreground hover:text-foreground"
	>
		← {$_('incidents.detail.back')}
	</a>
	<PageHeader title={$_('incidents.edit.title')} />

	{#if loading}
		<div class="max-w-2xl space-y-3">
			<Skeleton class="h-10 w-full" />
			<Skeleton class="h-10 w-full" />
		</div>
	{:else if notFound}
		<Alert>
			<AlertTitle>{$_('incidents.detail.not_found_title')}</AlertTitle>
			<AlertDescription>{$_('incidents.detail.not_found_description')}</AlertDescription>
		</Alert>
	{:else}
		<form class="max-w-2xl space-y-5" onsubmit={submit}>
			<div class="space-y-1.5">
				<Label for="i-title">{$_('incidents.fields.title')}</Label>
				<Input id="i-title" bind:value={title} maxlength={200} required />
			</div>
			<div class="space-y-1.5">
				<Label for="i-severity">{$_('incidents.fields.severity')}</Label>
				<Select.Root type="single" bind:value={severity}>
					<Select.Trigger id="i-severity" class="w-full">
						{$_(`incidents.severity.${severity}`)}
					</Select.Trigger>
					<Select.Content>
						<Select.Item value="minor">{$_('incidents.severity.minor')}</Select.Item>
						<Select.Item value="major">{$_('incidents.severity.major')}</Select.Item>
						<Select.Item value="critical">{$_('incidents.severity.critical')}</Select.Item>
					</Select.Content>
				</Select.Root>
			</div>
			<div class="flex gap-2">
				<Button type="submit" disabled={saving}>
					{saving ? $_('common.saving') : $_('incidents.edit.submit')}
				</Button>
				<Button
					type="button"
					variant="secondary"
					onclick={() => goto(`/incidents/${incidentId}`)}
				>
					{$_('common.cancel')}
				</Button>
			</div>
		</form>
	{/if}
</div>
