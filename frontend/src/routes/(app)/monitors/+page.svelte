<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<script lang="ts">
	import { onMount } from 'svelte';
	import { _ } from 'svelte-i18n';
	import { page } from '$app/state';
	import { Button } from '$lib/components/ui/button';
	import {
		Dialog,
		DialogContent,
		DialogHeader,
		DialogTitle,
		DialogDescription,
		DialogFooter
	} from '$lib/components/ui/dialog';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Alert, AlertTitle, AlertDescription } from '$lib/components/ui/alert';
	import PageHeader from '$lib/components/common/PageHeader.svelte';
	import MonitorsTable from '$lib/components/monitors/MonitorsTable.svelte';
	import MonitorDialog from '$lib/components/monitors/MonitorDialog.svelte';
	import { listMonitors, deleteMonitor, updateMonitor, type Monitor } from '$lib/api/monitors';
	import Pagination from '$lib/components/common/Pagination.svelte';
	import { toastError, toastSuccess } from '$lib/toast';

	let monitors = $state<Monitor[]>([]);
	let pageIdx = $state(1);
	const PAGE_SIZE = 25;
	let loading = $state(true);
	let loadError = $state<string | null>(null);

	let dialogOpen = $state(false);
	let dialogMode = $state<'create' | 'edit'>('create');
	let editing = $state<Monitor | null>(null);
	let preselectComponentId = $state<number | null>(null);

	let deleteTarget = $state<Monitor | null>(null);
	let deleteOpen = $state(false);
	let deleting = $state(false);

	async function refresh() {
		loading = true;
		loadError = null;
		try {
			monitors = await listMonitors();
		} catch (err) {
			loadError = $_('monitors.list.load_error');
			toastError(err, $_('monitors.list.load_error'));
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		const cidParam = page.url.searchParams.get('component_id');
		if (page.url.searchParams.has('new')) {
			preselectComponentId = cidParam ? Number(cidParam) : null;
			openCreate();
		}
		void refresh();
	});

	function openCreate() {
		dialogMode = 'create';
		editing = null;
		dialogOpen = true;
	}

	function openEdit(m: Monitor) {
		dialogMode = 'edit';
		editing = m;
		preselectComponentId = null;
		dialogOpen = true;
	}

	async function toggleEnabled(m: Monitor) {
		try {
			await updateMonitor(m.id, { enabled: !m.enabled });
			await refresh();
		} catch (err) {
			toastError(err, $_('common.error_generic'));
		}
	}

	function askDelete(m: Monitor) {
		deleteTarget = m;
		deleteOpen = true;
	}

	async function confirmDelete() {
		if (!deleteTarget) return;
		deleting = true;
		try {
			await deleteMonitor(deleteTarget.id);
			toastSuccess($_('monitors.delete.success'));
			deleteOpen = false;
			deleteTarget = null;
			await refresh();
		} catch (err) {
			toastError(err, $_('common.error_generic'));
		} finally {
			deleting = false;
		}
	}
</script>

<div class="p-6">
	<PageHeader title={$_('monitors.list.title')}>
		{#snippet actions()}
			<Button onclick={openCreate}>{$_('monitors.list.new_button')}</Button>
		{/snippet}
	</PageHeader>

	{#if loading && monitors.length === 0}
		<div class="space-y-2">
			{#each Array(5) as _, i (i)}
				<Skeleton class="h-12 w-full" />
			{/each}
		</div>
	{:else if loadError}
		<Alert variant="destructive">
			<AlertTitle>{$_('monitors.list.load_error')}</AlertTitle>
			<AlertDescription>{loadError}</AlertDescription>
		</Alert>
		<Button variant="outline" class="mt-4" onclick={refresh}>{$_('common.retry')}</Button>
	{:else if monitors.length === 0}
		<div class="flex flex-col items-center justify-center py-16 text-center">
			<p class="mb-4 text-muted-foreground">{$_('monitors.list.empty_description')}</p>
			<Button onclick={openCreate}>{$_('monitors.list.empty_cta')}</Button>
		</div>
	{:else}
		<MonitorsTable
			monitors={monitors.slice((pageIdx - 1) * PAGE_SIZE, pageIdx * PAGE_SIZE)}
			onEdit={openEdit}
			onToggleEnabled={toggleEnabled}
			onDelete={askDelete}
		/>
		<Pagination
			page={pageIdx}
			pageSize={PAGE_SIZE}
			total={monitors.length}
			onPageChange={(p) => (pageIdx = p)}
		/>
	{/if}
</div>

<MonitorDialog
	open={dialogOpen}
	mode={dialogMode}
	existing={editing}
	{preselectComponentId}
	onOpenChange={(v) => (dialogOpen = v)}
	onSaved={refresh}
/>

<Dialog open={deleteOpen} onOpenChange={(v) => (deleteOpen = v)}>
	<DialogContent class="sm:max-w-[420px]">
		<DialogHeader>
			<DialogTitle>{$_('monitors.delete.confirm_title')}</DialogTitle>
			<DialogDescription>
				{#if deleteTarget}
					{$_('monitors.delete.confirm_description', { values: { name: deleteTarget.name } })}
				{/if}
			</DialogDescription>
		</DialogHeader>
		<DialogFooter>
			<Button variant="secondary" onclick={() => (deleteOpen = false)}>
				{$_('common.cancel')}
			</Button>
			<Button variant="destructive" onclick={confirmDelete} disabled={deleting}>
				{deleting ? $_('common.deleting') : $_('monitors.delete.confirm_button')}
			</Button>
		</DialogFooter>
	</DialogContent>
</Dialog>
