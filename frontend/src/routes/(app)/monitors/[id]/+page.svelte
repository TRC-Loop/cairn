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
	import StatusBadge from '$lib/components/common/StatusBadge.svelte';
	import MonitorDialog from '$lib/components/monitors/MonitorDialog.svelte';
	import LatencyChart from '$lib/components/monitors/LatencyChart.svelte';
	import {
		getMonitor,
		getMonitorResults,
		deleteMonitor,
		type Monitor,
		type MonitorResult
	} from '$lib/api/monitors';
	import { ApiError } from '$lib/api/client';
	import { relativeTime, statusColorVar } from '$lib/utils';
	import { toastError, toastSuccess } from '$lib/toast';

	let monitor = $state<Monitor | null>(null);
	let results = $state<MonitorResult[]>([]);
	let loading = $state(true);
	let notFound = $state(false);

	let editOpen = $state(false);
	let deleteOpen = $state(false);
	let deleting = $state(false);

	const id = Number(page.params.id);

	async function load() {
		loading = true;
		notFound = false;
		try {
			const [m, r] = await Promise.all([getMonitor(id), getMonitorResults(id, 24)]);
			monitor = m;
			results = r;
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
		if (!monitor) return;
		deleting = true;
		try {
			await deleteMonitor(monitor.id);
			toastSuccess($_('monitors.delete.success'));
			await goto('/monitors');
		} catch (err) {
			toastError(err, $_('common.error_generic'));
		} finally {
			deleting = false;
		}
	}

	const recentResults = $derived(results.slice(-50).reverse());
</script>

<div class="p-6">
	<a
		href="/monitors"
		class="mb-4 inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
	>
		<IconArrowLeft size={14} />
		{$_('monitors.detail.back')}
	</a>

	{#if loading}
		<Skeleton class="mb-6 h-10 w-64" />
		<Skeleton class="mb-2 h-40 w-full" />
	{:else if notFound}
		<Alert variant="destructive">
			<AlertTitle>{$_('monitors.detail.not_found_title')}</AlertTitle>
			<AlertDescription>{$_('monitors.detail.not_found_description')}</AlertDescription>
		</Alert>
		<Button href="/monitors" variant="outline" class="mt-4">
			{$_('monitors.detail.back')}
		</Button>
	{:else if monitor}
		{@const m = monitor}
		<PageHeader title={m.name} description={$_(`monitors.types.${m.type}`)}>
			{#snippet actions()}
				<StatusBadge status={m.last_status} />
				<Button variant="secondary" onclick={() => (editOpen = true)}>
					{$_('monitors.detail.edit')}
				</Button>
				<Button variant="destructive" onclick={() => (deleteOpen = true)}>
					{$_('monitors.detail.delete')}
				</Button>
			{/snippet}
		</PageHeader>

		<div class="mb-6 rounded-md border border-border p-4">
			<h2 class="mb-3 text-sm font-medium text-foreground">
				{$_('monitors.detail.config_title')}
			</h2>
			<dl class="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-sm">
				<dt class="text-muted-foreground">{$_('monitors.fields.type')}</dt>
				<dd class="text-foreground">{m.type}</dd>
				<dt class="text-muted-foreground">{$_('monitors.fields.interval')}</dt>
				<dd class="text-foreground">{m.interval_seconds}s</dd>
				<dt class="text-muted-foreground">{$_('monitors.fields.timeout')}</dt>
				<dd class="text-foreground">{m.timeout_seconds}s</dd>
				<dt class="text-muted-foreground">{$_('monitors.fields.failure_threshold')}</dt>
				<dd class="text-foreground">{m.failure_threshold}</dd>
				<dt class="text-muted-foreground">{$_('monitors.fields.recovery_threshold')}</dt>
				<dd class="text-foreground">{m.recovery_threshold}</dd>
				{#if m.type === 'http'}
					<dt class="text-muted-foreground">{$_('monitors.fields.expected_status_codes')}</dt>
					<dd class="text-foreground">
						{(m.config?.expected_status_codes as string) ?? '200-299'}
					</dd>
				{/if}
				{#each Object.entries(m.config ?? {}) as [k, v] (k)}
					{#if k !== 'expected_status_codes'}
						<dt class="text-muted-foreground">{k}</dt>
						<dd class="break-all text-foreground">{String(v)}</dd>
					{/if}
				{/each}
				{#if m.push_token}
					<dt class="text-muted-foreground">push_token</dt>
					<dd class="break-all font-mono text-xs text-foreground">{m.push_token}</dd>
				{/if}
			</dl>
		</div>

		<div class="mb-6 rounded-md border border-border p-4">
			<h2 class="mb-3 text-sm font-medium text-foreground">
				{$_('monitors.detail.chart_title')}
			</h2>
			{#if results.length > 0}
				<LatencyChart {results} />
			{:else}
				<p class="text-sm text-muted-foreground">{$_('common.na')}</p>
			{/if}
		</div>

		<div class="rounded-md border border-border p-4">
			<h2 class="mb-3 text-sm font-medium text-foreground">
				{$_('monitors.detail.results_title')}
			</h2>
			{#if recentResults.length === 0}
				<p class="text-sm text-muted-foreground">{$_('common.na')}</p>
			{:else}
				<table class="w-full text-sm">
					<tbody>
						{#each recentResults as r (r.checked_at)}
							<tr class="border-b border-border last:border-b-0">
								<td class="py-1.5 text-muted-foreground">{relativeTime(r.checked_at)}</td>
								<td class="py-1.5">
									<span class="inline-flex items-center gap-1.5">
										<span
											class="inline-block h-1.5 w-1.5 rounded-full"
											style="background-color: {statusColorVar(r.status)};"
										></span>
										<span class="text-foreground">{r.status}</span>
									</span>
								</td>
								<td class="py-1.5 text-muted-foreground">
									{r.latency_ms != null ? `${r.latency_ms} ms` : $_('common.na')}
								</td>
								<td class="py-1.5 text-xs text-muted-foreground">{r.error_message ?? ''}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			{/if}
		</div>

		<MonitorDialog
			open={editOpen}
			mode="edit"
			existing={monitor}
			onOpenChange={(v) => (editOpen = v)}
			onSaved={load}
		/>

		<Dialog open={deleteOpen} onOpenChange={(v) => (deleteOpen = v)}>
			<DialogContent class="sm:max-w-[420px]">
				<DialogHeader>
					<DialogTitle>{$_('monitors.delete.confirm_title')}</DialogTitle>
					<DialogDescription>
						{$_('monitors.delete.confirm_description', { values: { name: monitor.name } })}
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
	{/if}
</div>
