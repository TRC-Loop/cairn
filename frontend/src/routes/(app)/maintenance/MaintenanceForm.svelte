<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<script lang="ts">
	import { onMount } from 'svelte';
	import { _ } from 'svelte-i18n';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { listComponents, type Component } from '$lib/api/components';
	import { toastError } from '$lib/toast';
	import type { MaintenanceState } from '$lib/api/maintenance';

	type Props = {
		mode: 'create' | 'edit';
		windowState?: MaintenanceState;
		title: string;
		description: string;
		startsAtLocal: string;
		endsAtLocal: string;
		selected: Set<number>;
		saving: boolean;
		onsubmit: () => void;
		oncancel: () => void;
	};

	let {
		mode,
		windowState = 'scheduled',
		title = $bindable(),
		description = $bindable(),
		startsAtLocal = $bindable(),
		endsAtLocal = $bindable(),
		selected = $bindable(),
		saving,
		onsubmit,
		oncancel
	}: Props = $props();

	let components = $state<Component[]>([]);
	let loadingComponents = $state(true);
	let search = $state('');

	const startsLocked = $derived(mode === 'edit' && windowState === 'in_progress');
	const componentsLocked = $derived(mode === 'edit' && windowState === 'in_progress');

	const filtered = $derived(
		components.filter((c) => c.name.toLowerCase().includes(search.trim().toLowerCase()))
	);

	const dateError = $derived.by(() => {
		if (!startsAtLocal || !endsAtLocal) return null;
		const s = new Date(startsAtLocal);
		const e = new Date(endsAtLocal);
		if (e <= s) return $_('maintenance.fields.ends_before_starts');
		return null;
	});

	onMount(async () => {
		try {
			components = await listComponents();
		} catch (err) {
			toastError(err, $_('common.error_generic'));
		} finally {
			loadingComponents = false;
		}
	});

	function toggle(id: number) {
		if (componentsLocked) return;
		const next = new Set(selected);
		if (next.has(id)) next.delete(id);
		else next.add(id);
		selected = next;
	}

	function submit(e: Event) {
		e.preventDefault();
		if (dateError) return;
		onsubmit();
	}
</script>

<form class="max-w-2xl space-y-5" onsubmit={submit}>
	<div class="space-y-1.5">
		<Label for="m-title">{$_('maintenance.fields.title')}</Label>
		<Input id="m-title" bind:value={title} maxlength={200} required />
	</div>

	<div class="space-y-1.5">
		<Label for="m-description">{$_('maintenance.fields.description')}</Label>
		<textarea
			id="m-description"
			bind:value={description}
			rows="4"
			maxlength={2000}
			class="flex w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm shadow-xs placeholder:text-muted-foreground focus-visible:border-ring focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring/50"
		></textarea>
		<p class="text-xs text-muted-foreground">{$_('maintenance.fields.description_help')}</p>
	</div>

	<div class="grid gap-4 sm:grid-cols-2">
		<div class="space-y-1.5">
			<Label for="m-starts">{$_('maintenance.fields.starts_at')}</Label>
			<Input
				id="m-starts"
				type="datetime-local"
				bind:value={startsAtLocal}
				required
				disabled={startsLocked}
				title={startsLocked ? $_('maintenance.fields.locked_in_progress') : ''}
			/>
			<p class="text-xs text-muted-foreground">{$_('maintenance.fields.starts_at_help')}</p>
		</div>
		<div class="space-y-1.5">
			<Label for="m-ends">{$_('maintenance.fields.ends_at')}</Label>
			<Input id="m-ends" type="datetime-local" bind:value={endsAtLocal} required />
			<p class="text-xs text-muted-foreground">{$_('maintenance.fields.ends_at_help')}</p>
		</div>
	</div>

	{#if dateError}
		<p class="text-sm text-destructive">{dateError}</p>
	{/if}

	<div class="space-y-1.5">
		<Label>{$_('maintenance.fields.affected_components')}</Label>
		{#if componentsLocked}
			<p class="text-xs text-muted-foreground">{$_('maintenance.fields.locked_in_progress')}</p>
		{/if}
		<Input
			type="text"
			placeholder={$_('components.attach_monitors.search_placeholder')}
			bind:value={search}
			disabled={loadingComponents || components.length === 0 || componentsLocked}
		/>
		<div class="max-h-[320px] overflow-y-auto rounded-md border border-border">
			{#if loadingComponents}
				<div class="space-y-1 p-2">
					{#each Array(4) as _, i (i)}
						<Skeleton class="h-8 w-full" />
					{/each}
				</div>
			{:else if components.length === 0}
				<p class="p-4 text-sm text-muted-foreground">
					{$_('maintenance.fields.no_components')}
				</p>
			{:else if filtered.length === 0}
				<p class="p-4 text-sm text-muted-foreground">{$_('common.no_results')}</p>
			{:else}
				<ul class="divide-y divide-border">
					{#each filtered as c (c.id)}
						<li>
							<label
								class="flex items-center gap-3 px-3 py-2 text-sm {componentsLocked
									? 'cursor-not-allowed opacity-60'
									: 'cursor-pointer hover:bg-accent'}"
							>
								<input
									type="checkbox"
									checked={selected.has(c.id)}
									disabled={componentsLocked}
									onchange={() => toggle(c.id)}
									class="rounded border-input"
								/>
								<span class="flex-1 text-foreground">{c.name}</span>
							</label>
						</li>
					{/each}
				</ul>
			{/if}
		</div>
	</div>

	<div class="flex gap-2">
		<Button type="submit" disabled={saving || !!dateError}>
			{saving ? $_('common.saving') : mode === 'create' ? $_('maintenance.create.submit') : $_('maintenance.edit.submit')}
		</Button>
		<Button type="button" variant="secondary" onclick={oncancel}>
			{$_('common.cancel')}
		</Button>
	</div>
</form>
