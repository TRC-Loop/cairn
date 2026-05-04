<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<script lang="ts">
	import { onMount } from 'svelte';
	import { _ } from 'svelte-i18n';
	import { goto } from '$app/navigation';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
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
		listIncidents,
		deleteIncident,
		type Incident,
		type IncidentListFilter,
		type IncidentSeverity
	} from '$lib/api/incidents';
	import { relativeTime } from '$lib/utils';
	import { toastError, toastSuccess } from '$lib/toast';

	let incidents = $state<Incident[]>([]);
	let loading = $state(true);
	let loadError = $state<string | null>(null);
	let tab = $state<IncidentListFilter>('active');
	let search = $state('');

	let deleteTarget = $state<Incident | null>(null);
	let deleteOpen = $state(false);
	let deleting = $state(false);

	async function refresh() {
		loading = true;
		loadError = null;
		try {
			const res = await listIncidents(tab);
			incidents = res.incidents.sort(
				(a, b) => new Date(b.started_at).getTime() - new Date(a.started_at).getTime()
			);
		} catch (err) {
			loadError = $_('incidents.list.load_error');
			toastError(err, $_('incidents.list.load_error'));
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		void refresh();
	});

	function setTab(next: IncidentListFilter) {
		if (tab === next) return;
		tab = next;
		void refresh();
	}

	const filtered = $derived.by(() => {
		const q = search.trim().toLowerCase();
		if (!q) return incidents;
		return incidents.filter((i) => i.title.toLowerCase().includes(q));
	});

	function severityColor(s: IncidentSeverity): string {
		switch (s) {
			case 'minor':
				return 'var(--status-degraded)';
			case 'major':
				return 'var(--status-down)';
			case 'critical':
				return 'var(--status-down)';
		}
	}

	function statusColor(s: string): string {
		switch (s) {
			case 'resolved':
				return 'var(--status-up)';
			case 'monitoring':
				return 'var(--status-degraded)';
			default:
				return 'var(--status-down)';
		}
	}

	function rowClick(i: Incident) {
		void goto(`/incidents/${i.id}`);
	}

	function askDelete(i: Incident) {
		deleteTarget = i;
		deleteOpen = true;
	}

	async function confirmDelete() {
		if (!deleteTarget) return;
		deleting = true;
		try {
			await deleteIncident(deleteTarget.id);
			toastSuccess($_('incidents.delete.success'));
			deleteOpen = false;
			deleteTarget = null;
			await refresh();
		} catch (err) {
			toastError(err, $_('common.error_generic'));
		} finally {
			deleting = false;
		}
	}

	function emptyText(): string {
		if (tab === 'active') return $_('incidents.list.empty_active');
		if (tab === 'resolved') return $_('incidents.list.empty_resolved');
		return $_('incidents.list.empty_all');
	}
</script>

<div class="p-6">
	<PageHeader title={$_('incidents.list.title')}>
		{#snippet actions()}
			<Button onclick={() => goto('/incidents/new')}>
				{$_('incidents.list.new_button')}
			</Button>
		{/snippet}
	</PageHeader>

	<div class="mb-4 flex items-center justify-between gap-3">
		<div class="flex items-center gap-1 rounded-md border border-border p-0.5">
			{#each [['active', $_('incidents.list.tab_active')], ['all', $_('incidents.list.tab_all')], ['resolved', $_('incidents.list.tab_resolved')]] as [key, label] (key)}
				<button
					type="button"
					class="rounded-sm px-3 py-1 text-sm transition-colors {tab === key
						? 'bg-accent text-foreground'
						: 'text-muted-foreground hover:text-foreground'}"
					onclick={() => setTab(key as IncidentListFilter)}
				>
					{label}
				</button>
			{/each}
		</div>
		<Input
			type="text"
			placeholder={$_('incidents.list.search_placeholder')}
			bind:value={search}
			class="max-w-xs"
		/>
	</div>

	{#if loading && incidents.length === 0}
		<div class="space-y-2">
			{#each Array(4) as _, i (i)}
				<Skeleton class="h-12 w-full" />
			{/each}
		</div>
	{:else if loadError}
		<Alert variant="destructive">
			<AlertTitle>{$_('incidents.list.load_error')}</AlertTitle>
			<AlertDescription>{loadError}</AlertDescription>
		</Alert>
		<Button variant="outline" class="mt-4" onclick={refresh}>{$_('common.retry')}</Button>
	{:else if filtered.length === 0}
		<div class="flex flex-col items-center justify-center py-16 text-center">
			<p class="mb-4 text-muted-foreground">{emptyText()}</p>
			{#if tab !== 'resolved'}
				<Button onclick={() => goto('/incidents/new')}>
					{$_('incidents.list.new_button')}
				</Button>
			{/if}
		</div>
	{:else}
		<div class="overflow-hidden rounded-md border border-border">
			<Table.Root>
				<Table.Header>
					<Table.Row class="bg-muted/40">
						<Table.Head class="w-24 px-3 py-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">
							{$_('incidents.list.col_severity')}
						</Table.Head>
						<Table.Head class="px-3 py-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">
							{$_('incidents.list.col_title')}
						</Table.Head>
						<Table.Head class="w-32 px-3 py-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">
							{$_('incidents.list.col_status')}
						</Table.Head>
						<Table.Head class="w-40 px-3 py-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">
							{$_('incidents.list.col_affected')}
						</Table.Head>
						<Table.Head class="w-20 px-3 py-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">
							{$_('incidents.list.col_source')}
						</Table.Head>
						<Table.Head class="w-32 px-3 py-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">
							{$_('incidents.list.col_started')}
						</Table.Head>
						<Table.Head class="w-32 px-3 py-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">
							{$_('incidents.list.col_resolved')}
						</Table.Head>
						<Table.Head class="w-10 px-3 py-2"></Table.Head>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each filtered as inc (inc.id)}
						{@const sevColor = severityColor(inc.severity)}
						{@const stColor = statusColor(inc.status)}
						<Table.Row class="cursor-pointer hover:bg-accent" onclick={() => rowClick(inc)}>
							<Table.Cell class="px-3 py-2">
								<Badge variant="outline" style="color: {sevColor}; border-color: {sevColor};">
									{$_(`incidents.severity.${inc.severity}`)}
								</Badge>
							</Table.Cell>
							<Table.Cell class="px-3 py-2 text-foreground">
								<span class="mr-2 font-mono text-xs text-muted-foreground">{inc.display_id}</span>
								{inc.title}
							</Table.Cell>
							<Table.Cell class="px-3 py-2">
								<Badge variant="outline" style="color: {stColor}; border-color: {stColor};">
									{$_(`incidents.status.${inc.status}`)}
								</Badge>
							</Table.Cell>
							<Table.Cell class="px-3 py-2 text-muted-foreground">
								{#if inc.triggering_check}
									<span class="truncate">{inc.triggering_check.name}{inc.affected_check_count > 1 ? ` +${inc.affected_check_count - 1}` : ''}</span>
								{:else}
									{$_('incidents.affected.count', { values: { count: inc.affected_check_count } })}
								{/if}
							</Table.Cell>
							<Table.Cell class="px-3 py-2 text-muted-foreground">
								{inc.auto_created ? $_('incidents.source.auto') : $_('incidents.source.manual')}
							</Table.Cell>
							<Table.Cell class="px-3 py-2 text-muted-foreground">
								{relativeTime(inc.started_at)}
							</Table.Cell>
							<Table.Cell class="px-3 py-2 text-muted-foreground">
								{inc.resolved_at ? relativeTime(inc.resolved_at) : $_('common.na')}
							</Table.Cell>
							<Table.Cell class="px-3 py-2 text-right" onclick={(e) => e.stopPropagation()}>
								<DropdownMenu>
									<DropdownMenuTrigger
										class="inline-flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground hover:bg-accent hover:text-foreground"
										aria-label={$_('incidents.list.col_actions')}
									>
										<IconDotsVertical size={14} />
									</DropdownMenuTrigger>
									<DropdownMenuContent align="end" class="w-[160px]">
										<DropdownMenuItem onclick={() => rowClick(inc)}>
											{$_('incidents.actions.view')}
										</DropdownMenuItem>
										<DropdownMenuItem onclick={() => askDelete(inc)}>
											{$_('incidents.actions.delete')}
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

<Dialog open={deleteOpen} onOpenChange={(v) => (deleteOpen = v)}>
	<DialogContent class="sm:max-w-[420px]">
		<DialogHeader>
			<DialogTitle>{$_('incidents.delete.confirm_title')}</DialogTitle>
			<DialogDescription>
				{#if deleteTarget}
					{$_('incidents.delete.confirm_description', { values: { title: deleteTarget.title } })}
				{/if}
			</DialogDescription>
		</DialogHeader>
		<DialogFooter>
			<Button variant="secondary" onclick={() => (deleteOpen = false)}>
				{$_('common.cancel')}
			</Button>
			<Button variant="destructive" onclick={confirmDelete} disabled={deleting}>
				{deleting ? $_('common.deleting') : $_('incidents.delete.confirm_button')}
			</Button>
		</DialogFooter>
	</DialogContent>
</Dialog>
