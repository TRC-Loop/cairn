<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<script lang="ts">
	import { onMount } from 'svelte';
	import { _ } from 'svelte-i18n';
	import PageHeader from '$lib/components/common/PageHeader.svelte';
	import { getDashboard, type DashboardSummary } from '$lib/api/dashboard';
	import { toastError } from '$lib/toast';

	let summary = $state<DashboardSummary | null>(null);
	let loading = $state(true);

	onMount(async () => {
		try {
			summary = await getDashboard();
		} catch (err) {
			toastError(err, $_('common.error_generic'));
		} finally {
			loading = false;
		}
	});

	function relative(iso: string): string {
		const t = new Date(iso).getTime();
		const diff = (Date.now() - t) / 1000;
		if (diff < 60) return `${Math.round(diff)}s ago`;
		if (diff < 3600) return `${Math.round(diff / 60)}m ago`;
		if (diff < 86400) return `${Math.round(diff / 3600)}h ago`;
		return `${Math.round(diff / 86400)}d ago`;
	}

	function activityLabel(type: string): string {
		switch (type) {
			case 'incident_opened':
				return $_('dashboard.activity.incident_opened');
			case 'incident_resolved':
				return $_('dashboard.activity.incident_resolved');
			case 'maintenance_started':
				return $_('dashboard.activity.maintenance_started');
			default:
				return type;
		}
	}

	function activityHref(type: string, id: number): string {
		if (type.startsWith('incident')) return `/incidents/${id}`;
		if (type.startsWith('maintenance')) return `/maintenance/${id}`;
		return '#';
	}
</script>

<div class="p-6">
	<PageHeader title={$_('dashboard.title')} />

	{#if loading}
		<p class="text-sm text-muted-foreground">{$_('common.loading')}</p>
	{:else if summary}
		<div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
			<a href="/monitors" class="rounded-md border border-border p-4 transition-colors hover:bg-accent">
				<p class="text-xs text-muted-foreground">{$_('dashboard.cards.monitors')}</p>
				<p class="mt-1 text-2xl font-medium text-foreground">{summary.monitors.total}</p>
				<p class="mt-1 text-xs text-muted-foreground">
					<span class="inline-flex items-center gap-1">
						<span class="inline-block h-1.5 w-1.5 rounded-full bg-emerald-500"></span>
						{summary.monitors.up} up
					</span>
					<span class="ml-2 inline-flex items-center gap-1">
						<span class="inline-block h-1.5 w-1.5 rounded-full bg-amber-500"></span>
						{summary.monitors.degraded}
					</span>
					<span class="ml-2 inline-flex items-center gap-1">
						<span class="inline-block h-1.5 w-1.5 rounded-full bg-red-500"></span>
						{summary.monitors.down}
					</span>
				</p>
			</a>

			<a href="/incidents" class="rounded-md border border-border p-4 transition-colors hover:bg-accent">
				<p class="text-xs text-muted-foreground">{$_('dashboard.cards.active_incidents')}</p>
				<p class="mt-1 text-2xl font-medium text-foreground">{summary.active_incidents.count}</p>
				<p class="mt-1 truncate text-xs text-muted-foreground">
					{summary.active_incidents.latest_title || $_('dashboard.cards.no_active_incidents')}
				</p>
			</a>

			<div class="rounded-md border border-border p-4">
				<p class="text-xs text-muted-foreground">{$_('dashboard.cards.uptime_24h')}</p>
				{#if summary.uptime_24h.has_current}
					<p class="mt-1 text-2xl font-medium text-foreground">
						{summary.uptime_24h.percentage.toFixed(1)}%
					</p>
					{#if summary.uptime_24h.has_previous}
						{@const delta = summary.uptime_24h.percentage - summary.uptime_24h.previous_24h_percentage}
						<p class="mt-1 text-xs {delta >= 0 ? 'text-emerald-500' : 'text-red-500'}">
							{delta >= 0 ? '↑' : '↓'} {Math.abs(delta).toFixed(2)}% {$_('dashboard.cards.from_yesterday')}
						</p>
					{/if}
				{:else}
					<p class="mt-1 text-sm text-muted-foreground">{$_('common.na')}</p>
				{/if}
			</div>

			<a href="/maintenance" class="rounded-md border border-border p-4 transition-colors hover:bg-accent">
				<p class="text-xs text-muted-foreground">{$_('dashboard.cards.maintenance')}</p>
				<p class="mt-1 text-2xl font-medium text-foreground">
					{summary.maintenance.in_progress_count}
				</p>
				<p class="mt-1 truncate text-xs text-muted-foreground">
					{summary.maintenance.next_window
						? `${summary.maintenance.next_window.title} · ${relative(summary.maintenance.next_window.starts_at)}`
						: $_('dashboard.cards.no_maintenance')}
				</p>
			</a>
		</div>

		<section class="mt-8">
			<h2 class="mb-3 text-sm font-medium text-foreground">
				{$_('dashboard.recent_activity')}
			</h2>
			{#if summary.recent_activity.length === 0}
				<p class="text-sm text-muted-foreground">{$_('dashboard.no_activity')}</p>
			{:else}
				<ul class="divide-y rounded-md border">
					{#each summary.recent_activity as a (a.type + '-' + a.id + '-' + a.timestamp)}
						<li class="flex items-center justify-between gap-3 px-3 py-2 text-sm">
							<div class="flex min-w-0 items-center gap-2">
								<span class="shrink-0 text-xs text-muted-foreground">{relative(a.timestamp)}</span>
								<span class="shrink-0 text-xs text-muted-foreground">·</span>
								<span class="shrink-0 text-xs text-muted-foreground">{activityLabel(a.type)}</span>
								<a href={activityHref(a.type, a.id)} class="truncate text-foreground hover:underline">
									{a.label}
								</a>
							</div>
						</li>
					{/each}
				</ul>
			{/if}
		</section>
	{/if}
</div>
