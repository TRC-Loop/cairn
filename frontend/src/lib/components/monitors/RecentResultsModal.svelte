<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<script lang="ts">
	import { _ } from 'svelte-i18n';
	import {
		Dialog,
		DialogContent,
		DialogHeader,
		DialogTitle,
		DialogFooter
	} from '$lib/components/ui/dialog';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { apiRequest } from '$lib/api/client';
	import type { MonitorResult } from '$lib/api/monitors';
	import { toastError } from '$lib/toast';
	import { relativeTime, statusColorVar } from '$lib/utils';

	type Props = {
		open: boolean;
		monitorId: number;
		onOpenChange: (v: boolean) => void;
	};

	let { open, monitorId, onOpenChange }: Props = $props();

	let rows = $state<MonitorResult[]>([]);
	let loading = $state(false);
	let loadingMore = $state(false);
	let exhausted = $state(false);
	let search = $state('');

	async function loadInitial() {
		loading = true;
		exhausted = false;
		rows = [];
		try {
			const { results } = await apiRequest<{ results: MonitorResult[] }>(
				`/api/monitors/${monitorId}/results?limit=50`
			);
			rows = results;
			if (results.length < 50) exhausted = true;
		} catch (err) {
			toastError(err, $_('common.error_generic'));
		} finally {
			loading = false;
		}
	}

	async function loadMore() {
		if (rows.length === 0 || exhausted) return;
		loadingMore = true;
		try {
			const before = rows[rows.length - 1].checked_at;
			const { results } = await apiRequest<{ results: MonitorResult[] }>(
				`/api/monitors/${monitorId}/results?limit=50&before=${encodeURIComponent(before)}`
			);
			rows = [...rows, ...results];
			if (results.length < 50) exhausted = true;
		} catch (err) {
			toastError(err, $_('common.error_generic'));
		} finally {
			loadingMore = false;
		}
	}

	$effect(() => {
		if (open) {
			search = '';
			loadInitial();
		}
	});

	const filtered = $derived(
		search.trim() === ''
			? rows
			: rows.filter(
					(r) =>
						r.status.toLowerCase().includes(search.toLowerCase()) ||
						(r.error_message ?? '').toLowerCase().includes(search.toLowerCase())
				)
	);
</script>

<Dialog {open} {onOpenChange}>
	<DialogContent class="sm:max-w-[640px]">
		<DialogHeader>
			<DialogTitle>{$_('monitors.detail.results_title')}</DialogTitle>
		</DialogHeader>
		<div class="space-y-3">
			<Input bind:value={search} placeholder={$_('common.search')} />
			<div class="max-h-[60vh] overflow-y-auto rounded-md border">
				{#if loading}
					<p class="px-3 py-4 text-center text-sm text-muted-foreground">
						{$_('common.loading')}
					</p>
				{:else if filtered.length === 0}
					<p class="px-3 py-4 text-center text-sm text-muted-foreground">
						{$_('common.na')}
					</p>
				{:else}
					<table class="w-full text-sm">
						<tbody>
							{#each filtered as r (r.checked_at)}
								<tr class="border-b last:border-b-0">
									<td class="px-2 py-1.5 text-xs text-muted-foreground whitespace-nowrap">
										{relativeTime(r.checked_at)}
									</td>
									<td class="px-2 py-1.5">
										<span class="inline-flex items-center gap-1.5">
											<span
												class="inline-block h-1.5 w-1.5 rounded-full"
												style="background-color: {statusColorVar(r.status)};"
											></span>
											<span class="text-foreground">{r.status}</span>
										</span>
									</td>
									<td class="px-2 py-1.5 text-xs text-muted-foreground whitespace-nowrap">
										{r.latency_ms != null ? `${r.latency_ms} ms` : ''}
									</td>
									<td
										class="max-w-[260px] truncate px-2 py-1.5 text-xs text-muted-foreground"
										title={r.error_message ?? ''}
									>
										{r.error_message ?? ''}
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				{/if}
			</div>
			{#if !exhausted && rows.length > 0}
				<div class="flex justify-center">
					<Button variant="secondary" size="sm" onclick={loadMore} disabled={loadingMore}>
						{loadingMore ? $_('common.loading') : $_('monitors.detail.load_more')}
					</Button>
				</div>
			{/if}
		</div>
		<DialogFooter>
			<Button variant="secondary" onclick={() => onOpenChange(false)}>
				{$_('common.close')}
			</Button>
		</DialogFooter>
	</DialogContent>
</Dialog>
