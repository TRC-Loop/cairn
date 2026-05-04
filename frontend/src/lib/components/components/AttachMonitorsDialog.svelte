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
	import { listMonitors, updateMonitor, type Monitor } from '$lib/api/monitors';
	import { toastError } from '$lib/toast';

	type Props = {
		open: boolean;
		componentId: number;
		componentName: string;
		onOpenChange: (v: boolean) => void;
		onAttached: () => void;
	};

	let { open, componentId, componentName, onOpenChange, onAttached }: Props = $props();

	let monitors = $state<Monitor[]>([]);
	let loading = $state(false);
	let saving = $state(false);
	let search = $state('');
	let selected = $state<Set<number>>(new Set());

	$effect(() => {
		if (open) {
			void loadMonitors();
		}
	});

	async function loadMonitors() {
		loading = true;
		search = '';
		selected = new Set();
		try {
			monitors = await listMonitors();
		} catch (err) {
			toastError(err, $_('common.error_generic'));
			monitors = [];
		} finally {
			loading = false;
		}
	}

	const eligible = $derived(monitors.filter((m) => m.component_id !== componentId));

	const filtered = $derived(
		eligible.filter((m) => m.name.toLowerCase().includes(search.trim().toLowerCase()))
	);

	function toggle(id: number) {
		const next = new Set(selected);
		if (next.has(id)) next.delete(id);
		else next.add(id);
		selected = next;
	}

	async function commit() {
		if (selected.size === 0) return;
		saving = true;
		const ids = [...selected];
		const results = await Promise.allSettled(
			ids.map((id) => updateMonitor(id, { component_id: componentId }))
		);
		const failures = results
			.map((r, i) => ({ r, id: ids[i] }))
			.filter((x) => x.r.status === 'rejected');
		for (const f of failures) {
			const m = monitors.find((x) => x.id === f.id);
			toastError(
				(f.r as PromiseRejectedResult).reason,
				m ? `Failed to attach "${m.name}"` : 'Failed to attach monitor'
			);
		}
		saving = false;
		onAttached();
		onOpenChange(false);
	}
</script>

<Dialog {open} {onOpenChange}>
	<DialogContent class="sm:max-w-[480px]">
		<DialogHeader>
			<DialogTitle>
				{$_('components.attach_monitors.title', { values: { name: componentName } })}
			</DialogTitle>
		</DialogHeader>

		<div class="space-y-3 py-2">
			<Input
				type="text"
				placeholder={$_('components.attach_monitors.search_placeholder')}
				bind:value={search}
				disabled={loading || eligible.length === 0}
			/>

			<div class="max-h-[400px] overflow-y-auto rounded-md border border-border">
				{#if loading}
					<p class="p-4 text-sm text-muted-foreground">{$_('common.loading')}</p>
				{:else if monitors.length === 0}
					<p class="p-4 text-sm text-muted-foreground">
						{$_('components.attach_monitors.no_monitors')}
					</p>
				{:else if eligible.length === 0}
					<p class="p-4 text-sm text-muted-foreground">
						{$_('components.attach_monitors.empty')}
					</p>
				{:else if filtered.length === 0}
					<p class="p-4 text-sm text-muted-foreground">{$_('common.no_results')}</p>
				{:else}
					<ul class="divide-y divide-border">
						{#each filtered as m (m.id)}
							<li>
								<label
									class="flex cursor-pointer items-center gap-3 px-3 py-2 text-sm hover:bg-accent"
								>
									<input
										type="checkbox"
										checked={selected.has(m.id)}
										onchange={() => toggle(m.id)}
										class="rounded border-input"
									/>
									<span class="flex-1 text-foreground">{m.name}</span>
									<span
										class="rounded-md border border-border px-1.5 py-0.5 text-xs uppercase text-muted-foreground"
									>
										{m.type}
									</span>
								</label>
							</li>
						{/each}
					</ul>
				{/if}
			</div>
		</div>

		<DialogFooter class="sm:justify-between">
			<a
				href={`/monitors/new?component_id=${componentId}`}
				class="text-xs text-muted-foreground hover:text-foreground hover:underline"
			>
				{$_('components.attach_monitors.create_new')}
			</a>
			<div class="flex gap-2">
				<Button variant="secondary" onclick={() => onOpenChange(false)} disabled={saving}>
					{$_('common.cancel')}
				</Button>
				<Button onclick={commit} disabled={selected.size === 0 || saving}>
					{saving
						? $_('common.saving')
						: $_('components.attach_monitors.add_n', { values: { n: selected.size } })}
				</Button>
			</div>
		</DialogFooter>
	</DialogContent>
</Dialog>
