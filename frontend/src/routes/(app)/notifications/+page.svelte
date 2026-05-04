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
	import NotificationChannelDialog from '$lib/components/notifications/NotificationChannelDialog.svelte';
	import {
		listChannels,
		deleteChannel,
		testChannel,
		type NotificationChannel
	} from '$lib/api/notifications';
	import { toastError, toastSuccess } from '$lib/toast';

	let channels = $state<NotificationChannel[]>([]);
	let loading = $state(true);
	let loadError = $state<string | null>(null);

	let dialogOpen = $state(false);
	let dialogMode = $state<'create' | 'edit'>('create');
	let editing = $state<NotificationChannel | null>(null);

	let deleteTarget = $state<NotificationChannel | null>(null);
	let deleteOpen = $state(false);
	let deleting = $state(false);

	async function refresh() {
		loading = true;
		loadError = null;
		try {
			const list = await listChannels();
			channels = list.sort((a, b) => a.name.localeCompare(b.name));
		} catch (err) {
			loadError = $_('notifications.list.load_error');
			toastError(err, $_('notifications.list.load_error'));
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		if (page.url.searchParams.has('new')) openCreate();
		void refresh();
	});

	function openCreate() {
		dialogMode = 'create';
		editing = null;
		dialogOpen = true;
	}

	function openEdit(c: NotificationChannel) {
		dialogMode = 'edit';
		editing = c;
		dialogOpen = true;
	}

	function askDelete(c: NotificationChannel) {
		deleteTarget = c;
		deleteOpen = true;
	}

	async function confirmDelete() {
		if (!deleteTarget) return;
		deleting = true;
		try {
			await deleteChannel(deleteTarget.id);
			toastSuccess($_('notifications.delete.success'));
			deleteOpen = false;
			deleteTarget = null;
			await refresh();
		} catch (err) {
			toastError(err, $_('common.error_generic'));
		} finally {
			deleting = false;
		}
	}

	async function sendTest(c: NotificationChannel) {
		try {
			await testChannel(c.id);
			toastSuccess($_('notifications.test.queued'));
		} catch (err) {
			toastError(err, $_('common.error_generic'));
		}
	}

	function rowClick(c: NotificationChannel) {
		void goto(`/notifications/${c.id}`);
	}
</script>

<div class="p-6">
	<PageHeader title={$_('notifications.list.title')}>
		{#snippet actions()}
			<Button onclick={openCreate}>{$_('notifications.list.new_button')}</Button>
		{/snippet}
	</PageHeader>

	{#if loading && channels.length === 0}
		<div class="space-y-2">
			{#each Array(4) as _, i (i)}
				<Skeleton class="h-12 w-full" />
			{/each}
		</div>
	{:else if loadError}
		<Alert variant="destructive">
			<AlertTitle>{$_('notifications.list.load_error')}</AlertTitle>
			<AlertDescription>{loadError}</AlertDescription>
		</Alert>
		<Button variant="outline" class="mt-4" onclick={refresh}>{$_('common.retry')}</Button>
	{:else if channels.length === 0}
		<div class="flex flex-col items-center justify-center py-16 text-center">
			<p class="mb-4 text-muted-foreground">{$_('notifications.list.empty_description')}</p>
			<Button onclick={openCreate}>{$_('notifications.list.empty_cta')}</Button>
		</div>
	{:else}
		<div class="overflow-hidden rounded-md border border-border">
			<Table.Root>
				<Table.Header>
					<Table.Row class="bg-muted/40">
						<Table.Head class="px-3 py-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">
							{$_('notifications.list.col_name')}
						</Table.Head>
						<Table.Head class="px-3 py-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">
							{$_('notifications.list.col_type')}
						</Table.Head>
						<Table.Head class="px-3 py-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">
							{$_('notifications.list.col_status')}
						</Table.Head>
						<Table.Head class="px-3 py-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">
							{$_('notifications.list.col_used_by')}
						</Table.Head>
						<Table.Head class="w-10 px-3 py-2"></Table.Head>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each channels as c (c.id)}
						<Table.Row class="cursor-pointer hover:bg-accent" onclick={() => rowClick(c)}>
							<Table.Cell class="px-3 py-2 text-foreground">{c.name}</Table.Cell>
							<Table.Cell class="px-3 py-2 text-muted-foreground">
								{$_(`notifications.types.${c.type}`)}
							</Table.Cell>
							<Table.Cell class="px-3 py-2 text-muted-foreground">
								{c.enabled
									? $_('notifications.status.enabled')
									: $_('notifications.status.disabled')}
							</Table.Cell>
							<Table.Cell class="px-3 py-2 text-muted-foreground">
								{c.used_by_check_count}
							</Table.Cell>
							<Table.Cell class="px-3 py-2 text-right" onclick={(e) => e.stopPropagation()}>
								<DropdownMenu>
									<DropdownMenuTrigger
										class="inline-flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground hover:bg-accent hover:text-foreground"
										aria-label={$_('common.actions')}
									>
										<IconDotsVertical size={14} />
									</DropdownMenuTrigger>
									<DropdownMenuContent align="end" class="w-[180px]">
										<DropdownMenuItem onclick={() => openEdit(c)}>
											{$_('common.edit')}
										</DropdownMenuItem>
										<DropdownMenuItem onclick={() => sendTest(c)} disabled={!c.enabled}>
											{$_('notifications.actions.send_test')}
										</DropdownMenuItem>
										<DropdownMenuItem onclick={() => askDelete(c)}>
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

<NotificationChannelDialog
	open={dialogOpen}
	mode={dialogMode}
	existing={editing}
	onOpenChange={(v) => (dialogOpen = v)}
	onSaved={refresh}
/>

<Dialog open={deleteOpen} onOpenChange={(v) => (deleteOpen = v)}>
	<DialogContent class="sm:max-w-[420px]">
		<DialogHeader>
			<DialogTitle>{$_('notifications.delete.confirm_title')}</DialogTitle>
			<DialogDescription>
				{#if deleteTarget}
					{$_('notifications.delete.confirm_description', {
						values: { name: deleteTarget.name }
					})}
				{/if}
			</DialogDescription>
		</DialogHeader>
		<DialogFooter>
			<Button variant="secondary" onclick={() => (deleteOpen = false)}>
				{$_('common.cancel')}
			</Button>
			<Button variant="destructive" onclick={confirmDelete} disabled={deleting}>
				{deleting ? $_('common.deleting') : $_('notifications.delete.confirm_button')}
			</Button>
		</DialogFooter>
	</DialogContent>
</Dialog>
