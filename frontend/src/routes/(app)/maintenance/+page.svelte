<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<script lang="ts">
	import { onMount } from 'svelte';
	import { _ } from 'svelte-i18n';
	import { goto } from '$app/navigation';
	import { Button } from '$lib/components/ui/button';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Alert, AlertTitle, AlertDescription } from '$lib/components/ui/alert';
	import { Badge } from '$lib/components/ui/badge';
	import * as Table from '$lib/components/ui/table';
	import {
		Dialog,
		DialogContent,
		DialogHeader,
		DialogTitle,
		DialogDescription,
		DialogFooter
	} from '$lib/components/ui/dialog';
	import {
		DropdownMenu,
		DropdownMenuTrigger,
		DropdownMenuContent,
		DropdownMenuItem
	} from '$lib/components/ui/dropdown-menu';
	import { IconDotsVertical } from '@tabler/icons-svelte';
	import PageHeader from '$lib/components/common/PageHeader.svelte';
	import {
		listMaintenance,
		deleteMaintenance,
		cancelMaintenance,
		endMaintenanceNow,
		type MaintenanceWindow,
		type MaintenanceState
	} from '$lib/api/maintenance';
	import { toastError, toastSuccess } from '$lib/toast';

	type Tab = 'upcoming' | 'in_progress' | 'past' | 'all';

	let tab = $state<Tab>('upcoming');
	let windows = $state<MaintenanceWindow[]>([]);
	let loading = $state(true);
	let loadError = $state<string | null>(null);

	let confirmTarget = $state<MaintenanceWindow | null>(null);
	let confirmAction = $state<'delete' | 'cancel' | 'end' | null>(null);
	let confirmOpen = $state(false);
	let confirmBusy = $state(false);

	let initialResolved = false;

	async function refresh() {
		loading = true;
		loadError = null;
		try {
			let res;
			if (tab === 'upcoming') {
				res = await listMaintenance({ upcoming: true });
			} else if (tab === 'in_progress') {
				res = await listMaintenance({ status: 'in_progress' });
			} else if (tab === 'past') {
				res = await listMaintenance({ pastDays: 90, limit: 200 });
			} else {
				res = await listMaintenance({ status: 'all', limit: 200 });
			}
			windows = res.maintenance;
		} catch (err) {
			loadError = $_('maintenance.list.load_error');
			toastError(err, $_('maintenance.list.load_error'));
		} finally {
			loading = false;
		}
	}

	onMount(async () => {
		// Pick a sensible default tab
		try {
			const upcoming = await listMaintenance({ upcoming: true });
			if (upcoming.maintenance.length > 0) {
				tab = 'upcoming';
				windows = upcoming.maintenance;
				loading = false;
				initialResolved = true;
				return;
			}
			const inProgress = await listMaintenance({ status: 'in_progress' });
			if (inProgress.maintenance.length > 0) {
				tab = 'in_progress';
				windows = inProgress.maintenance;
				loading = false;
				initialResolved = true;
				return;
			}
			const past = await listMaintenance({ pastDays: 90, limit: 200 });
			if (past.maintenance.length > 0) {
				tab = 'past';
				windows = past.maintenance;
				loading = false;
				initialResolved = true;
				return;
			}
		} catch (err) {
			loadError = $_('maintenance.list.load_error');
		}
		// All empty: stay on upcoming (default tab) so empty CTA shows
		windows = [];
		loading = false;
		initialResolved = true;
	});

	function setTab(next: Tab) {
		if (tab === next || !initialResolved) return;
		tab = next;
		void refresh();
	}

	function stateColor(s: MaintenanceState): string {
		switch (s) {
			case 'scheduled':
				return 'var(--muted-foreground)';
			case 'in_progress':
				return 'var(--status-degraded)';
			case 'completed':
				return 'var(--muted-foreground)';
			case 'cancelled':
				return 'var(--muted-foreground)';
		}
	}

	function formatRange(startsAt: string, endsAt: string): string {
		const s = new Date(startsAt);
		const e = new Date(endsAt);
		const sameDay = s.toDateString() === e.toDateString();
		const dateFmt: Intl.DateTimeFormatOptions = { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' };
		const timeFmt: Intl.DateTimeFormatOptions = { hour: '2-digit', minute: '2-digit' };
		if (sameDay) {
			return `${s.toLocaleString(undefined, dateFmt)} → ${e.toLocaleTimeString(undefined, timeFmt)}`;
		}
		return `${s.toLocaleString(undefined, dateFmt)} → ${e.toLocaleString(undefined, dateFmt)}`;
	}

	function affectedSummary(w: MaintenanceWindow): string {
		const names = w.affected_component_names;
		if (!names || names.length === 0) return $_('common.na');
		if (names.length === 1) return names[0];
		return `${names[0]} +${names.length - 1}`;
	}

	function rowClick(w: MaintenanceWindow) {
		void goto(`/maintenance/${w.id}`);
	}

	function ask(action: 'delete' | 'cancel' | 'end', w: MaintenanceWindow) {
		confirmTarget = w;
		confirmAction = action;
		confirmOpen = true;
	}

	async function confirm() {
		if (!confirmTarget || !confirmAction) return;
		confirmBusy = true;
		try {
			if (confirmAction === 'delete') {
				await deleteMaintenance(confirmTarget.id);
				toastSuccess($_('maintenance.delete.success'));
			} else if (confirmAction === 'cancel') {
				await cancelMaintenance(confirmTarget.id);
				toastSuccess($_('maintenance.cancel.success'));
			} else if (confirmAction === 'end') {
				await endMaintenanceNow(confirmTarget.id);
				toastSuccess($_('maintenance.end_now.success'));
			}
			confirmOpen = false;
			confirmTarget = null;
			confirmAction = null;
			await refresh();
		} catch (err) {
			toastError(err, $_('common.error_generic'));
		} finally {
			confirmBusy = false;
		}
	}

	function emptyText(): string {
		if (tab === 'upcoming') return $_('maintenance.list.empty_upcoming');
		if (tab === 'in_progress') return $_('maintenance.list.empty_in_progress');
		if (tab === 'past') return $_('maintenance.list.empty_past');
		return $_('maintenance.list.empty_all');
	}

	function confirmTitle(): string {
		if (confirmAction === 'delete') return $_('maintenance.delete.confirm_title');
		if (confirmAction === 'cancel') return $_('maintenance.cancel.confirm_title');
		if (confirmAction === 'end') return $_('maintenance.end_now.confirm_title');
		return '';
	}

	function confirmBody(): string {
		if (confirmAction === 'delete') return $_('maintenance.delete.confirm_description');
		if (confirmAction === 'cancel') return $_('maintenance.cancel.confirm_description');
		if (confirmAction === 'end') return $_('maintenance.end_now.confirm_description');
		return '';
	}
</script>

<div class="p-6">
	<PageHeader title={$_('maintenance.list.title')}>
		{#snippet actions()}
			<Button onclick={() => goto('/maintenance/new')}>
				{$_('maintenance.list.new_button')}
			</Button>
		{/snippet}
	</PageHeader>

	<div class="mb-4 flex items-center gap-1 rounded-md border border-border p-0.5 w-fit">
		{#each [['upcoming', $_('maintenance.list.tab_upcoming')], ['in_progress', $_('maintenance.list.tab_in_progress')], ['past', $_('maintenance.list.tab_past')], ['all', $_('maintenance.list.tab_all')]] as [key, label] (key)}
			<button
				type="button"
				class="rounded-sm px-3 py-1 text-sm transition-colors {tab === key
					? 'bg-accent text-foreground'
					: 'text-muted-foreground hover:text-foreground'}"
				onclick={() => setTab(key as Tab)}
			>
				{label}
			</button>
		{/each}
	</div>

	{#if loading && windows.length === 0}
		<div class="space-y-2">
			{#each Array(3) as _, i (i)}
				<Skeleton class="h-12 w-full" />
			{/each}
		</div>
	{:else if loadError}
		<Alert variant="destructive">
			<AlertTitle>{$_('maintenance.list.load_error')}</AlertTitle>
			<AlertDescription>{loadError}</AlertDescription>
		</Alert>
		<Button variant="outline" class="mt-4" onclick={refresh}>{$_('common.retry')}</Button>
	{:else if windows.length === 0}
		<div class="flex flex-col items-center justify-center py-16 text-center">
			<p class="mb-4 text-muted-foreground">{emptyText()}</p>
			{#if tab === 'upcoming' || tab === 'all'}
				<Button onclick={() => goto('/maintenance/new')}>
					{$_('maintenance.list.new_button')}
				</Button>
			{/if}
		</div>
	{:else}
		<div class="overflow-hidden rounded-md border border-border">
			<Table.Root>
				<Table.Header>
					<Table.Row class="bg-muted/40">
						<Table.Head class="w-32 px-3 py-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">
							{$_('maintenance.list.col_state')}
						</Table.Head>
						<Table.Head class="px-3 py-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">
							{$_('maintenance.list.col_title')}
						</Table.Head>
						<Table.Head class="w-72 px-3 py-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">
							{$_('maintenance.list.col_window')}
						</Table.Head>
						<Table.Head class="w-48 px-3 py-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">
							{$_('maintenance.list.col_affected')}
						</Table.Head>
						<Table.Head class="w-10 px-3 py-2"></Table.Head>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each windows as w (w.id)}
						{@const sc = stateColor(w.state)}
						<Table.Row class="cursor-pointer hover:bg-accent" onclick={() => rowClick(w)}>
							<Table.Cell class="px-3 py-2">
								<Badge
									variant="outline"
									style="color: {sc}; border-color: {sc};"
									class={w.state === 'cancelled' ? 'line-through' : ''}
								>
									{$_(`maintenance.state.${w.state}`)}
								</Badge>
							</Table.Cell>
							<Table.Cell class="px-3 py-2 text-foreground">{w.title}</Table.Cell>
							<Table.Cell class="px-3 py-2 text-muted-foreground">
								{formatRange(w.starts_at, w.ends_at)}
							</Table.Cell>
							<Table.Cell class="px-3 py-2 text-muted-foreground">
								{affectedSummary(w)}
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
										<DropdownMenuItem onclick={() => rowClick(w)}>
											{$_('maintenance.actions.view')}
										</DropdownMenuItem>
										{#if w.state === 'scheduled'}
											<DropdownMenuItem onclick={() => goto(`/maintenance/${w.id}/edit`)}>
												{$_('maintenance.actions.edit')}
											</DropdownMenuItem>
											<DropdownMenuItem onclick={() => ask('cancel', w)}>
												{$_('maintenance.actions.cancel')}
											</DropdownMenuItem>
											<DropdownMenuItem onclick={() => ask('delete', w)}>
												{$_('maintenance.actions.delete')}
											</DropdownMenuItem>
										{:else if w.state === 'in_progress'}
											<DropdownMenuItem onclick={() => goto(`/maintenance/${w.id}/edit`)}>
												{$_('maintenance.actions.edit')}
											</DropdownMenuItem>
											<DropdownMenuItem onclick={() => ask('end', w)}>
												{$_('maintenance.actions.end_now')}
											</DropdownMenuItem>
										{:else if w.state === 'cancelled'}
											<DropdownMenuItem onclick={() => ask('delete', w)}>
												{$_('maintenance.actions.delete')}
											</DropdownMenuItem>
										{/if}
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

<Dialog open={confirmOpen} onOpenChange={(v) => (confirmOpen = v)}>
	<DialogContent class="sm:max-w-[440px]">
		<DialogHeader>
			<DialogTitle>{confirmTitle()}</DialogTitle>
			<DialogDescription>{confirmBody()}</DialogDescription>
		</DialogHeader>
		<DialogFooter>
			<Button variant="secondary" onclick={() => (confirmOpen = false)}>
				{$_('common.cancel')}
			</Button>
			<Button variant="destructive" onclick={confirm} disabled={confirmBusy}>
				{confirmBusy ? $_('common.loading') : $_('common.confirm')}
			</Button>
		</DialogFooter>
	</DialogContent>
</Dialog>
