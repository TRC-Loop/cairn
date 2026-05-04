<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<script lang="ts">
	import { onMount } from 'svelte';
	import { _ } from 'svelte-i18n';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Alert, AlertTitle, AlertDescription } from '$lib/components/ui/alert';
	import PageHeader from '$lib/components/common/PageHeader.svelte';
	import MaintenanceForm from '../../MaintenanceForm.svelte';
	import {
		getMaintenance,
		patchMaintenance,
		type MaintenanceState
	} from '$lib/api/maintenance';
	import { toastError, toastSuccess } from '$lib/toast';

	const id = $derived(parseInt($page.params.id ?? '', 10));

	let title = $state('');
	let description = $state('');
	let startsAtLocal = $state('');
	let endsAtLocal = $state('');
	let selected = $state<Set<number>>(new Set());
	let originalSelected = $state<Set<number>>(new Set());
	let originalStartsAt = $state('');

	let windowState = $state<MaintenanceState>('scheduled');
	let loading = $state(true);
	let loadError = $state(false);
	let saving = $state(false);

	function utcToLocalInput(iso: string): string {
		const d = new Date(iso);
		const pad = (n: number) => n.toString().padStart(2, '0');
		return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
	}
	function localToUTC(local: string): string {
		return new Date(local).toISOString();
	}

	onMount(async () => {
		try {
			const detail = await getMaintenance(id);
			const w = detail.window;
			if (w.state === 'completed' || w.state === 'cancelled') {
				void goto(`/maintenance/${id}`);
				return;
			}
			title = w.title;
			description = w.description;
			startsAtLocal = utcToLocalInput(w.starts_at);
			endsAtLocal = utcToLocalInput(w.ends_at);
			selected = new Set(w.affected_component_ids);
			originalSelected = new Set(w.affected_component_ids);
			originalStartsAt = w.starts_at;
			windowState = w.state;
		} catch (err) {
			loadError = true;
			toastError(err, $_('common.error_generic'));
		} finally {
			loading = false;
		}
	});

	async function submit() {
		if (!title.trim()) {
			toastError(null, $_('common.required'));
			return;
		}
		saving = true;
		try {
			const patch: Parameters<typeof patchMaintenance>[1] = {
				title: title.trim(),
				description: description.trim(),
				ends_at: localToUTC(endsAtLocal)
			};
			if (windowState === 'scheduled') {
				patch.starts_at = localToUTC(startsAtLocal);
				const a = [...originalSelected].sort();
				const b = [...selected].sort();
				const changed = a.length !== b.length || a.some((v, i) => v !== b[i]);
				if (changed) patch.affected_component_ids = [...selected];
			}
			await patchMaintenance(id, patch);
			toastSuccess($_('maintenance.edit.success'));
			void goto(`/maintenance/${id}`);
		} catch (err) {
			toastError(err, $_('common.error_generic'));
		} finally {
			saving = false;
		}
	}
</script>

<div class="p-6">
	<a href={`/maintenance/${id}`} class="mb-4 inline-block text-sm text-muted-foreground hover:text-foreground">
		← {$_('maintenance.detail.back')}
	</a>
	<PageHeader title={$_('maintenance.edit.title')} />

	{#if loading}
		<Skeleton class="h-64 w-full max-w-2xl" />
	{:else if loadError}
		<Alert variant="destructive">
			<AlertTitle>{$_('maintenance.detail.not_found_title')}</AlertTitle>
			<AlertDescription>{$_('maintenance.detail.not_found_description')}</AlertDescription>
		</Alert>
	{:else}
		<MaintenanceForm
			mode="edit"
			{windowState}
			bind:title
			bind:description
			bind:startsAtLocal
			bind:endsAtLocal
			bind:selected
			{saving}
			onsubmit={submit}
			oncancel={() => goto(`/maintenance/${id}`)}
		/>
	{/if}
</div>
