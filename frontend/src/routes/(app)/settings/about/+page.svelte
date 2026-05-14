<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<script lang="ts">
	import { onMount } from 'svelte';
	import { _ } from 'svelte-i18n';
	import { versionInfo } from '$lib/stores/version';
	import { getDBStats, type DBStats } from '$lib/api/dbStats';

	let stats = $state<DBStats | null>(null);
	let statsError = $state(false);

	onMount(async () => {
		versionInfo.load();
		try {
			stats = await getDBStats();
		} catch {
			statsError = true;
		}
	});

	function formatBytes(n: number): string {
		if (n < 1024) return `${n} B`;
		const units = ['KB', 'MB', 'GB', 'TB'];
		let v = n / 1024;
		let i = 0;
		while (v >= 1024 && i < units.length - 1) {
			v /= 1024;
			i++;
		}
		return `${v.toFixed(v >= 10 ? 0 : 1)} ${units[i]}`;
	}

	function healthPercent(bytes: number): number {
		const mb = 1024 * 1024;
		const gb = 1024 * mb;
		if (bytes < 100 * mb) return Math.max(5, Math.round((bytes / (100 * mb)) * 25));
		if (bytes < 500 * mb) return 25 + Math.round(((bytes - 100 * mb) / (400 * mb)) * 25);
		if (bytes < 2 * gb) return 50 + Math.round(((bytes - 500 * mb) / (1.5 * gb)) * 30);
		if (bytes < 5 * gb) return 80 + Math.round(((bytes - 2 * gb) / (3 * gb)) * 20);
		return 100;
	}

	const ROW_LABELS: Record<string, string> = {
		checks: 'monitors',
		check_results: 'results',
		check_results_hourly: 'results (hourly)',
		check_results_daily: 'results (daily)',
		incidents: 'incidents',
		users: 'users',
		components: 'components',
		status_pages: 'status pages',
		maintenance_windows: 'maintenance'
	};
</script>

<div class="space-y-8">
	<section>
		<h2 class="mb-2 text-base font-medium">{$_('settings.about.section')}</h2>
		<p class="mb-4 text-sm text-muted-foreground">{$_('settings.about.tagline')}</p>
		<dl class="grid grid-cols-[max-content_1fr] gap-x-6 gap-y-2 text-sm">
			<dt class="text-muted-foreground">{$_('settings.about.version')}</dt>
			<dd class="font-mono text-foreground">{$versionInfo.version}</dd>
			<dt class="text-muted-foreground">{$_('settings.about.license')}</dt>
			<dd>AGPL-3.0-or-later</dd>
			<dt class="text-muted-foreground">{$_('settings.about.source')}</dt>
			<dd>
				<a
					class="text-primary hover:underline"
					href="https://github.com/TRC-Loop/cairn"
					target="_blank"
					rel="noopener noreferrer">github.com/TRC-Loop/cairn</a
				>
			</dd>
		</dl>
	</section>

	{#if stats && !statsError}
		{@const pct = healthPercent(stats.size_bytes)}
		{@const barColor =
			stats.health === 'good'
				? 'bg-emerald-500'
				: stats.health === 'caution'
					? 'bg-amber-500'
					: 'bg-red-500'}
		{@const statusKey = `settings.about.database.status_${stats.health}`}
		<section>
			<h2 class="mb-2 text-base font-medium">{$_('settings.about.database.title')}</h2>
			<dl class="mb-4 grid grid-cols-[max-content_1fr] gap-x-6 gap-y-2 text-sm">
				<dt class="text-muted-foreground">{$_('settings.about.database.path')}</dt>
				<dd class="font-mono text-foreground">{stats.path}</dd>
				<dt class="text-muted-foreground">{$_('settings.about.database.size')}</dt>
				<dd class="font-mono text-foreground">{formatBytes(stats.size_bytes)}</dd>
			</dl>

			<div class="mb-2 h-2 w-full overflow-hidden rounded bg-muted">
				<div class="h-full {barColor}" style="width: {pct}%"></div>
			</div>
			<p class="text-xs text-muted-foreground">{$_(statusKey)}</p>

			<details class="mt-4">
				<summary class="cursor-pointer text-xs text-muted-foreground hover:text-foreground">
					{$_('settings.about.database.row_counts')}
				</summary>
				<table class="mt-2 text-xs">
					<tbody>
						{#each Object.entries(stats.rows) as [tbl, count] (tbl)}
							<tr>
								<td class="pr-6 text-muted-foreground">{ROW_LABELS[tbl] || tbl}</td>
								<td class="font-mono text-foreground">{count.toLocaleString()}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</details>
		</section>
	{/if}

	<section>
		<h2 class="mb-2 text-base font-medium">{$_('settings.about.privacy_title')}</h2>
		<p class="text-sm text-muted-foreground">{$_('settings.about.privacy_body')}</p>
	</section>
</div>
