<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<script lang="ts">
	import { onMount } from 'svelte';
	import { _ } from 'svelte-i18n';
	import { goto } from '$app/navigation';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Select from '$lib/components/ui/select';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import PageHeader from '$lib/components/common/PageHeader.svelte';
	import { createIncident, type IncidentSeverity } from '$lib/api/incidents';
	import { listMonitors, type Monitor } from '$lib/api/monitors';
	import { toastError, toastSuccess } from '$lib/toast';

	let title = $state('');
	let severity = $state<string>('minor');
	let initialMessage = $state('');
	let selected = $state<Set<number>>(new Set());
	let search = $state('');

	let monitors = $state<Monitor[]>([]);
	let loadingMonitors = $state(true);
	let saving = $state(false);

	const filtered = $derived(
		monitors.filter((m) => m.name.toLowerCase().includes(search.trim().toLowerCase()))
	);

	onMount(async () => {
		try {
			monitors = await listMonitors();
		} catch (err) {
			toastError(err, $_('common.error_generic'));
		} finally {
			loadingMonitors = false;
		}
	});

	function toggle(id: number) {
		const next = new Set(selected);
		if (next.has(id)) next.delete(id);
		else next.add(id);
		selected = next;
	}

	async function submit(e: Event) {
		e.preventDefault();
		const t = title.trim();
		const msg = initialMessage.trim();
		if (!t) {
			toastError(null, $_('common.required'));
			return;
		}
		if (!msg) {
			toastError(null, $_('incidents.create.message_required'));
			return;
		}
		if (selected.size === 0) {
			toastError(null, $_('incidents.create.pick_monitors'));
			return;
		}
		saving = true;
		try {
			const { incident } = await createIncident({
				title: t,
				severity: severity as IncidentSeverity,
				initial_message: msg,
				affected_check_ids: [...selected]
			});
			toastSuccess($_('incidents.create.success'));
			void goto(`/incidents/${incident.id}`);
		} catch (err) {
			toastError(err, $_('common.error_generic'));
		} finally {
			saving = false;
		}
	}
</script>

<div class="p-6">
	<a href="/incidents" class="mb-4 inline-block text-sm text-muted-foreground hover:text-foreground">
		← {$_('incidents.detail.back')}
	</a>
	<PageHeader title={$_('incidents.create.title')} />

	<form class="max-w-2xl space-y-5" onsubmit={submit}>
		<div class="space-y-1.5">
			<Label for="i-title">{$_('incidents.fields.title')}</Label>
			<Input id="i-title" bind:value={title} maxlength={200} required />
		</div>

		<div class="space-y-1.5">
			<Label for="i-severity">{$_('incidents.fields.severity')}</Label>
			<Select.Root type="single" bind:value={severity}>
				<Select.Trigger id="i-severity" class="w-full">
					{$_(`incidents.severity.${severity}`)}
				</Select.Trigger>
				<Select.Content>
					<Select.Item value="minor">{$_('incidents.severity.minor')}</Select.Item>
					<Select.Item value="major">{$_('incidents.severity.major')}</Select.Item>
					<Select.Item value="critical">{$_('incidents.severity.critical')}</Select.Item>
				</Select.Content>
			</Select.Root>
		</div>

		<div class="space-y-1.5">
			<Label for="i-message">{$_('incidents.fields.initial_message')}</Label>
			<textarea
				id="i-message"
				bind:value={initialMessage}
				rows="3"
				maxlength={2000}
				class="flex w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm shadow-xs placeholder:text-muted-foreground focus-visible:border-ring focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring/50"
			></textarea>
		</div>

		<div class="space-y-1.5">
			<Label>{$_('incidents.fields.affected_checks')}</Label>
			<Input
				type="text"
				placeholder={$_('components.attach_monitors.search_placeholder')}
				bind:value={search}
				disabled={loadingMonitors || monitors.length === 0}
			/>
			<div class="max-h-[320px] overflow-y-auto rounded-md border border-border">
				{#if loadingMonitors}
					<div class="space-y-1 p-2">
						{#each Array(4) as _, i (i)}
							<Skeleton class="h-8 w-full" />
						{/each}
					</div>
				{:else if monitors.length === 0}
					<p class="p-4 text-sm text-muted-foreground">
						{$_('components.attach_monitors.no_monitors')}
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

		<div class="flex gap-2">
			<Button type="submit" disabled={saving}>
				{saving ? $_('common.saving') : $_('incidents.create.submit')}
			</Button>
			<Button type="button" variant="secondary" onclick={() => goto('/incidents')}>
				{$_('common.cancel')}
			</Button>
		</div>
	</form>
</div>
