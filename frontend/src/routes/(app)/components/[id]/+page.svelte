<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<script lang="ts">
	import { onMount } from 'svelte';
	import { _ } from 'svelte-i18n';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { Button } from '$lib/components/ui/button';
	import {
		Dialog,
		DialogContent,
		DialogHeader,
		DialogTitle,
		DialogDescription,
		DialogFooter
	} from '$lib/components/ui/dialog';
	import { Alert, AlertTitle, AlertDescription } from '$lib/components/ui/alert';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { IconArrowLeft } from '@tabler/icons-svelte';
	import PageHeader from '$lib/components/common/PageHeader.svelte';
	import ComponentDialog from '$lib/components/components/ComponentDialog.svelte';
	import AttachMonitorsDialog from '$lib/components/components/AttachMonitorsDialog.svelte';
	import { getComponent, deleteComponent, type Component } from '$lib/api/components';
	import { ApiError } from '$lib/api/client';
	import { statusColorVar } from '$lib/utils';
	import type { Monitor } from '$lib/api/monitors';
	import { toastError, toastSuccess } from '$lib/toast';

	let component = $state<Component | null>(null);
	let monitors = $state<Monitor[]>([]);
	let loading = $state(true);
	let notFound = $state(false);

	let editOpen = $state(false);
	let deleteOpen = $state(false);
	let deleting = $state(false);
	let attachOpen = $state(false);

	const id = Number(page.params.id);

	async function load() {
		loading = true;
		notFound = false;
		try {
			const res = await getComponent(id);
			component = res.component;
			monitors = res.checks;
		} catch (err) {
			if (err instanceof ApiError && err.status === 404) {
				notFound = true;
			} else {
				toastError(err, $_('common.error_generic'));
			}
		} finally {
			loading = false;
		}
	}

	onMount(load);

	async function confirmDelete() {
		if (!component) return;
		deleting = true;
		try {
			await deleteComponent(component.id);
			toastSuccess($_('components.delete.success'));
			await goto('/components');
		} catch (err) {
			toastError(err, $_('common.error_generic'));
		} finally {
			deleting = false;
		}
	}

	function openAttach() {
		if (!component) return;
		attachOpen = true;
	}
</script>

<div class="p-6">
	<a
		href="/components"
		class="mb-4 inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
	>
		<IconArrowLeft size={14} />
		{$_('components.detail.back')}
	</a>

	{#if loading}
		<Skeleton class="mb-6 h-10 w-64" />
		<Skeleton class="mb-2 h-40 w-full" />
	{:else if notFound}
		<Alert variant="destructive">
			<AlertTitle>{$_('components.detail.not_found_title')}</AlertTitle>
			<AlertDescription>{$_('components.detail.not_found_description')}</AlertDescription>
		</Alert>
		<Button href="/components" variant="outline" class="mt-4">
			{$_('components.detail.back')}
		</Button>
	{:else if component}
		{@const c = component}
		<PageHeader title={c.name} description={c.description ?? undefined}>
			{#snippet actions()}
				<Button variant="secondary" onclick={() => (editOpen = true)}>
					{$_('components.detail.edit')}
				</Button>
				<Button variant="destructive" onclick={() => (deleteOpen = true)}>
					{$_('components.detail.delete')}
				</Button>
			{/snippet}
		</PageHeader>

		<div class="rounded-md border border-border p-4">
			<div class="mb-3 flex items-center justify-between">
				<h2 class="text-sm font-medium text-foreground">
					{$_('components.detail.monitors_title')}
				</h2>
				<Button size="sm" variant="secondary" onclick={openAttach}>
					{monitors.length === 0
						? $_('components.detail.add_monitor_first')
						: $_('components.detail.add_monitor')}
				</Button>
			</div>

			{#if monitors.length === 0}
				<p class="text-sm text-muted-foreground">{$_('components.detail.monitors_empty')}</p>
			{:else}
				<table class="w-full text-sm">
					<tbody>
						{#each monitors as m (m.id)}
							<tr class="border-b border-border last:border-b-0">
								<td class="py-1.5">
									<span
										class="inline-block h-1.5 w-1.5 rounded-full"
										style="background-color: {statusColorVar(m.last_status)};"
									></span>
								</td>
								<td class="py-1.5 text-foreground">
									<a href={`/monitors/${m.id}`} class="hover:underline">{m.name}</a>
								</td>
								<td class="py-1.5 text-muted-foreground">{m.type}</td>
								<td class="py-1.5 text-right text-muted-foreground">
									{m.last_latency_ms != null ? `${m.last_latency_ms} ms` : $_('common.na')}
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			{/if}
		</div>

		<ComponentDialog
			open={editOpen}
			mode="edit"
			existing={component}
			onOpenChange={(v) => (editOpen = v)}
			onSaved={load}
		/>

		<AttachMonitorsDialog
			open={attachOpen}
			componentId={component.id}
			componentName={component.name}
			onOpenChange={(v) => (attachOpen = v)}
			onAttached={load}
		/>

		<Dialog open={deleteOpen} onOpenChange={(v) => (deleteOpen = v)}>
			<DialogContent class="sm:max-w-[420px]">
				<DialogHeader>
					<DialogTitle>{$_('components.delete.confirm_title')}</DialogTitle>
					<DialogDescription>
						{$_('components.delete.confirm_description', { values: { name: component.name } })}
					</DialogDescription>
				</DialogHeader>
				<DialogFooter>
					<Button variant="secondary" onclick={() => (deleteOpen = false)}>
						{$_('common.cancel')}
					</Button>
					<Button variant="destructive" onclick={confirmDelete} disabled={deleting}>
						{deleting ? $_('common.deleting') : $_('components.delete.confirm_button')}
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	{/if}
</div>
