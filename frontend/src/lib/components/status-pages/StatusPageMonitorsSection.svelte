<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<script lang="ts">
	import { onMount } from 'svelte';
	import { _ } from 'svelte-i18n';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import {
		Dialog,
		DialogContent,
		DialogHeader,
		DialogTitle,
		DialogFooter
	} from '$lib/components/ui/dialog';
	import {
		listStatusPageMonitors,
		setStatusPageMonitors,
		type DirectMonitor
	} from '$lib/api/statusPages';
	import { listMonitors, type Monitor } from '$lib/api/monitors';
	import { toastError, toastSuccess } from '$lib/toast';

	type Props = { statusPageId: number };
	let { statusPageId }: Props = $props();

	let linked = $state<DirectMonitor[]>([]);
	let loading = $state(true);
	let pickerOpen = $state(false);
	let allMonitors = $state<Monitor[]>([]);
	let search = $state('');
	let saving = $state(false);

	async function load() {
		loading = true;
		try {
			linked = await listStatusPageMonitors(statusPageId);
		} catch (err) {
			toastError(err, $_('common.error_generic'));
		} finally {
			loading = false;
		}
	}

	onMount(load);

	async function openPicker() {
		try {
			allMonitors = await listMonitors();
			search = '';
			pickerOpen = true;
		} catch (err) {
			toastError(err, $_('common.error_generic'));
		}
	}

	const candidates = $derived(
		allMonitors.filter(
			(m) =>
				!linked.some((l) => l.id === m.id) &&
				(search.trim() === '' || m.name.toLowerCase().includes(search.toLowerCase()))
		)
	);

	async function addMonitor(m: Monitor) {
		saving = true;
		try {
			const nextIds = [...linked.map((l) => l.id), m.id];
			await setStatusPageMonitors(statusPageId, nextIds);
			await load();
			toastSuccess($_('status_pages.direct_monitors.added'));
		} catch (err) {
			toastError(err, $_('common.error_generic'));
		} finally {
			saving = false;
		}
	}

	async function removeMonitor(m: DirectMonitor) {
		if (!confirm($_('status_pages.direct_monitors.confirm_remove'))) return;
		saving = true;
		try {
			const nextIds = linked.filter((l) => l.id !== m.id).map((l) => l.id);
			await setStatusPageMonitors(statusPageId, nextIds);
			linked = linked.filter((l) => l.id !== m.id);
			toastSuccess($_('status_pages.direct_monitors.removed'));
		} catch (err) {
			toastError(err, $_('common.error_generic'));
		} finally {
			saving = false;
		}
	}

	async function move(idx: number, dir: -1 | 1) {
		const j = idx + dir;
		if (j < 0 || j >= linked.length) return;
		const next = [...linked];
		[next[idx], next[j]] = [next[j], next[idx]];
		saving = true;
		try {
			await setStatusPageMonitors(
				statusPageId,
				next.map((m) => m.id)
			);
			linked = next;
		} catch (err) {
			toastError(err, $_('common.error_generic'));
		} finally {
			saving = false;
		}
	}
</script>

<section class="space-y-3">
	<div class="flex items-start justify-between gap-4">
		<div>
			<h2 class="text-base font-medium">{$_('status_pages.direct_monitors.title')}</h2>
			<p class="text-xs text-muted-foreground">{$_('status_pages.direct_monitors.help')}</p>
		</div>
		<Button variant="secondary" size="sm" onclick={openPicker}>
			{$_('status_pages.direct_monitors.add')}
		</Button>
	</div>

	{#if loading}
		<p class="text-sm text-muted-foreground">…</p>
	{:else if linked.length === 0}
		<p class="text-sm text-muted-foreground">{$_('status_pages.direct_monitors.empty')}</p>
	{:else}
		<ul class="divide-y rounded-md border">
			{#each linked as m, idx (m.id)}
				<li class="flex items-center justify-between gap-2 px-3 py-2">
					<span class="truncate text-sm">{m.name}</span>
					<div class="flex shrink-0 items-center gap-1">
						<Button variant="ghost" size="sm" disabled={idx === 0 || saving} onclick={() => move(idx, -1)}>↑</Button>
						<Button variant="ghost" size="sm" disabled={idx === linked.length - 1 || saving} onclick={() => move(idx, 1)}>↓</Button>
						<Button variant="ghost" size="sm" disabled={saving} onclick={() => removeMonitor(m)}>
							{$_('status_pages.direct_monitors.remove')}
						</Button>
					</div>
				</li>
			{/each}
		</ul>
	{/if}
</section>

<Dialog open={pickerOpen} onOpenChange={(v) => (pickerOpen = v)}>
	<DialogContent class="sm:max-w-[480px]">
		<DialogHeader>
			<DialogTitle>{$_('status_pages.direct_monitors.add')}</DialogTitle>
		</DialogHeader>
		<div class="space-y-3">
			<Input bind:value={search} placeholder={$_('common.search')} />
			<div class="max-h-[50vh] overflow-y-auto rounded-md border">
				{#if candidates.length === 0}
					<p class="px-3 py-4 text-center text-sm text-muted-foreground">
						{$_('status_pages.direct_monitors.no_results')}
					</p>
				{:else}
					<ul class="divide-y">
						{#each candidates as m (m.id)}
							<li class="flex items-center justify-between px-3 py-2">
								<span class="truncate text-sm">{m.name}</span>
								<Button
									variant="ghost"
									size="sm"
									disabled={saving}
									onclick={() => addMonitor(m)}
								>
									+
								</Button>
							</li>
						{/each}
					</ul>
				{/if}
			</div>
		</div>
		<DialogFooter>
			<Button variant="secondary" onclick={() => (pickerOpen = false)}>
				{$_('common.close')}
			</Button>
		</DialogFooter>
	</DialogContent>
</Dialog>
