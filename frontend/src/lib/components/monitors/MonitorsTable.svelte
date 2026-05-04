<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<script lang="ts">
	import { _ } from 'svelte-i18n';
	import { goto } from '$app/navigation';
	import * as Table from '$lib/components/ui/table';
	import * as Tooltip from '$lib/components/ui/tooltip';
	import {
		DropdownMenu,
		DropdownMenuTrigger,
		DropdownMenuContent,
		DropdownMenuItem
	} from '$lib/components/ui/dropdown-menu';
	import { IconDotsVertical } from '@tabler/icons-svelte';
	import { relativeTime, statusColorVar } from '$lib/utils';
	import type { Monitor } from '$lib/api/monitors';

	type Props = {
		monitors: Monitor[];
		onEdit: (m: Monitor) => void;
		onToggleEnabled: (m: Monitor) => void;
		onDelete: (m: Monitor) => void;
	};

	let { monitors, onEdit, onToggleEnabled, onDelete }: Props = $props();

	const NAME_TRUNCATE = 40;

	function rowClick(m: Monitor) {
		goto(`/monitors/${m.id}`);
	}
</script>

<div class="overflow-hidden rounded-md border border-border">
	<Tooltip.Provider delayDuration={200}>
		<Table.Root>
			<Table.Header>
				<Table.Row class="bg-muted/40">
					<Table.Head class="w-6 px-3 py-2"></Table.Head>
					<Table.Head class="px-3 py-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">
						{$_('monitors.list.col_name')}
					</Table.Head>
					<Table.Head class="px-3 py-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">
						{$_('monitors.list.col_type')}
					</Table.Head>
					<Table.Head class="px-3 py-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">
						{$_('monitors.list.col_last_checked')}
					</Table.Head>
					<Table.Head class="px-3 py-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">
						{$_('monitors.list.col_latency')}
					</Table.Head>
					<Table.Head class="w-10 px-3 py-2"></Table.Head>
				</Table.Row>
			</Table.Header>
			<Table.Body>
				{#each monitors as m (m.id)}
					<Table.Row class="cursor-pointer hover:bg-accent" onclick={() => rowClick(m)}>
						<Table.Cell class="px-3 py-2">
							<span
								class="inline-block h-2 w-2 rounded-full"
								style="background-color: {statusColorVar(m.last_status)};"
								aria-label={m.last_status}
							></span>
						</Table.Cell>
						<Table.Cell class="px-3 py-2 text-foreground">
							{#if m.name.length > NAME_TRUNCATE}
								<Tooltip.Root>
									<Tooltip.Trigger class="block max-w-[240px] truncate text-left">
										{m.name}
									</Tooltip.Trigger>
									<Tooltip.Content>{m.name}</Tooltip.Content>
								</Tooltip.Root>
							{:else}
								<span>{m.name}</span>
							{/if}
							{#if !m.enabled}
								<span class="ml-2 text-xs text-muted-foreground">({$_('monitors.status.paused')})</span>
							{/if}
						</Table.Cell>
						<Table.Cell class="px-3 py-2 text-muted-foreground">{m.type}</Table.Cell>
						<Table.Cell class="px-3 py-2 text-muted-foreground">
							{m.last_checked_at ? relativeTime(m.last_checked_at) : $_('common.na')}
						</Table.Cell>
						<Table.Cell class="px-3 py-2 text-muted-foreground">
							{m.last_latency_ms != null ? `${m.last_latency_ms} ms` : $_('common.na')}
						</Table.Cell>
						<Table.Cell class="px-3 py-2 text-right" onclick={(e) => e.stopPropagation()}>
							<DropdownMenu>
								<DropdownMenuTrigger
									class="inline-flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground hover:bg-accent hover:text-foreground"
									aria-label={$_('monitors.list.col_actions')}
								>
									<IconDotsVertical size={14} />
								</DropdownMenuTrigger>
								<DropdownMenuContent align="end" class="w-[160px]">
									<DropdownMenuItem onclick={() => onEdit(m)}>
										{$_('monitors.actions.edit')}
									</DropdownMenuItem>
									<DropdownMenuItem onclick={() => onToggleEnabled(m)}>
										{m.enabled ? $_('monitors.actions.disable') : $_('monitors.actions.enable')}
									</DropdownMenuItem>
									<DropdownMenuItem onclick={() => onDelete(m)} class="text-destructive">
										{$_('monitors.actions.delete')}
									</DropdownMenuItem>
								</DropdownMenuContent>
							</DropdownMenu>
						</Table.Cell>
					</Table.Row>
				{/each}
			</Table.Body>
		</Table.Root>
	</Tooltip.Provider>
</div>
