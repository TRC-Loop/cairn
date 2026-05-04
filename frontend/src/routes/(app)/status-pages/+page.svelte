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
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Alert, AlertTitle, AlertDescription } from '$lib/components/ui/alert';
	import * as Table from '$lib/components/ui/table';
	import {
		DropdownMenu,
		DropdownMenuTrigger,
		DropdownMenuContent,
		DropdownMenuItem
	} from '$lib/components/ui/dropdown-menu';
	import { IconDotsVertical } from '@tabler/icons-svelte';
	import PageHeader from '$lib/components/common/PageHeader.svelte';
	import StatusPageDialog from '$lib/components/status-pages/StatusPageDialog.svelte';
	import {
		listStatusPages,
		deleteStatusPage,
		type StatusPage
	} from '$lib/api/statusPages';
	import { toastError, toastSuccess } from '$lib/toast';
	import { relativeTime } from '$lib/utils';

	let pages = $state<StatusPage[]>([]);
	let loading = $state(true);
	let loadError = $state<string | null>(null);

	let dialogOpen = $state(false);

	let deleteTarget = $state<StatusPage | null>(null);
	let deleteOpen = $state(false);
	let deleting = $state(false);

	async function refresh() {
		loading = true;
		loadError = null;
		try {
			pages = await listStatusPages();
		} catch (err) {
			loadError = $_('status_pages.list.load_error');
			toastError(err, $_('status_pages.list.load_error'));
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		if (page.url.searchParams.has('new')) {
			dialogOpen = true;
		}
		void refresh();
	});

	function rowClick(p: StatusPage) {
		void goto(`/status-pages/${p.id}`);
	}

	function askDelete(p: StatusPage) {
		deleteTarget = p;
		deleteOpen = true;
	}

	async function confirmDelete() {
		if (!deleteTarget) return;
		deleting = true;
		try {
			await deleteStatusPage(deleteTarget.id);
			toastSuccess($_('status_pages.delete.success'));
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
	<PageHeader title={$_('status_pages.list.title')}>
		{#snippet actions()}
			<Button onclick={() => (dialogOpen = true)}>
				{$_('status_pages.list.new_button')}
			</Button>
		{/snippet}
	</PageHeader>

	{#if loading && pages.length === 0}
		<div class="space-y-2">
			{#each Array(3) as _, i (i)}
				<Skeleton class="h-12 w-full" />
			{/each}
		</div>
	{:else if loadError}
		<Alert variant="destructive">
			<AlertTitle>{$_('status_pages.list.load_error')}</AlertTitle>
			<AlertDescription>{loadError}</AlertDescription>
		</Alert>
		<Button variant="outline" class="mt-4" onclick={refresh}>{$_('common.retry')}</Button>
	{:else if pages.length === 0}
		<div class="flex flex-col items-center justify-center py-16 text-center">
			<p class="mb-4 text-muted-foreground">{$_('status_pages.list.empty_description')}</p>
			<Button onclick={() => (dialogOpen = true)}>
				{$_('status_pages.list.empty_cta')}
			</Button>
		</div>
	{:else}
		<div class="overflow-hidden rounded-md border border-border">
			<Table.Root>
				<Table.Header>
					<Table.Row class="bg-muted/40">
						<Table.Head class="px-3 py-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">
							{$_('status_pages.list.col_title')}
						</Table.Head>
						<Table.Head class="px-3 py-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">
							{$_('status_pages.list.col_slug')}
						</Table.Head>
						<Table.Head class="px-3 py-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">
							{$_('status_pages.list.col_default')}
						</Table.Head>
						<Table.Head class="px-3 py-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">
							{$_('status_pages.list.col_created')}
						</Table.Head>
						<Table.Head class="w-10 px-3 py-2"></Table.Head>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each pages as p (p.id)}
						<Table.Row class="cursor-pointer hover:bg-accent" onclick={() => rowClick(p)}>
							<Table.Cell class="px-3 py-2 text-foreground">{p.title}</Table.Cell>
							<Table.Cell class="px-3 py-2 font-mono text-xs text-muted-foreground">
								{p.slug}
							</Table.Cell>
							<Table.Cell class="px-3 py-2">
								{#if p.is_default}
									<span class="inline-flex items-center rounded-md border border-border px-2 py-0.5 text-xs text-muted-foreground">
										{$_('status_pages.list.default_badge')}
									</span>
								{:else}
									<span class="text-xs text-muted-foreground">{$_('common.na')}</span>
								{/if}
							</Table.Cell>
							<Table.Cell class="px-3 py-2 text-muted-foreground">
								{relativeTime(p.created_at)}
							</Table.Cell>
							<Table.Cell class="px-3 py-2 text-right" onclick={(e) => e.stopPropagation()}>
								<DropdownMenu>
									<DropdownMenuTrigger
										class="inline-flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground hover:bg-accent hover:text-foreground"
										aria-label={$_('status_pages.list.col_actions')}
									>
										<IconDotsVertical size={14} />
									</DropdownMenuTrigger>
									<DropdownMenuContent align="end" class="w-[180px]">
										<DropdownMenuItem onclick={() => goto(`/status-pages/${p.id}`)}>
											{$_('common.edit')}
										</DropdownMenuItem>
										<DropdownMenuItem onclick={() => window.open(`/p/${p.slug}`, '_blank')}>
											{$_('status_pages.list.view_public')}
										</DropdownMenuItem>
										<DropdownMenuItem onclick={() => askDelete(p)}>
											{$_('common.delete')}
										</DropdownMenuItem>
									</DropdownMenuContent>
								</DropdownMenu>
							</Table.Cell>
						</Table.Row>
					{/each}
				</Table.Body>
			</Table.Root>
		</div>
	{/if}
</div>

<StatusPageDialog
	open={dialogOpen}
	mode="create"
	onOpenChange={(v) => (dialogOpen = v)}
	onSaved={refresh}
/>

<Dialog open={deleteOpen} onOpenChange={(v) => (deleteOpen = v)}>
	<DialogContent class="sm:max-w-[420px]">
		<DialogHeader>
			<DialogTitle>{$_('status_pages.delete.confirm_title')}</DialogTitle>
			<DialogDescription>
				{#if deleteTarget}
					{$_('status_pages.delete.confirm_description', { values: { title: deleteTarget.title } })}
				{/if}
			</DialogDescription>
		</DialogHeader>
		<DialogFooter>
			<Button variant="secondary" onclick={() => (deleteOpen = false)}>
				{$_('common.cancel')}
			</Button>
			<Button variant="destructive" onclick={confirmDelete} disabled={deleting}>
				{deleting ? $_('common.deleting') : $_('status_pages.delete.confirm_button')}
			</Button>
		</DialogFooter>
	</DialogContent>
</Dialog>
