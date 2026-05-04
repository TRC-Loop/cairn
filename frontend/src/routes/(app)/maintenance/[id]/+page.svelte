<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<script lang="ts">
	import { onMount } from 'svelte';
	import { _ } from 'svelte-i18n';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { Button } from '$lib/components/ui/button';
	import { Badge } from '$lib/components/ui/badge';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Alert, AlertTitle, AlertDescription } from '$lib/components/ui/alert';
	import {
		Dialog,
		DialogContent,
		DialogHeader,
		DialogTitle,
		DialogDescription,
		DialogFooter
	} from '$lib/components/ui/dialog';
	import PageHeader from '$lib/components/common/PageHeader.svelte';
	import {
		getMaintenance,
		cancelMaintenance,
		endMaintenanceNow,
		deleteMaintenance,
		type MaintenanceDetail,
		type MaintenanceState
	} from '$lib/api/maintenance';
	import { toastError, toastSuccess } from '$lib/toast';
	import { relativeTime } from '$lib/utils';

	const id = $derived(parseInt($page.params.id ?? '', 10));

	let detail = $state<MaintenanceDetail | null>(null);
	let loading = $state(true);
	let loadError = $state(false);

	let confirmAction = $state<'cancel' | 'end' | 'delete' | null>(null);
	let confirmOpen = $state(false);
	let confirmBusy = $state(false);

	async function refresh() {
		loading = true;
		loadError = false;
		try {
			detail = await getMaintenance(id);
		} catch (err) {
			loadError = true;
			toastError(err, $_('common.error_generic'));
		} finally {
			loading = false;
		}
	}

	onMount(() => void refresh());

	function stateColor(s: MaintenanceState): string {
		switch (s) {
			case 'in_progress':
				return 'var(--status-degraded)';
			default:
				return 'var(--muted-foreground)';
		}
	}

	function durationText(startsAt: string, endsAt: string): string {
		const ms = new Date(endsAt).getTime() - new Date(startsAt).getTime();
		if (ms <= 0) return $_('common.na');
		const mins = Math.round(ms / 60000);
		if (mins < 60) return `${mins} min`;
		const hrs = Math.floor(mins / 60);
		const rem = mins % 60;
		if (rem === 0) return `${hrs} h`;
		return `${hrs} h ${rem} min`;
	}

	function relativeWindow(w: NonNullable<MaintenanceDetail>['window']): string {
		if (w.state === 'scheduled') {
			return $_('maintenance.detail.starts_in', { values: { rel: relativeTime(w.starts_at) } });
		}
		if (w.state === 'in_progress') {
			return $_('maintenance.detail.ends_in', { values: { rel: relativeTime(w.ends_at) } });
		}
		if (w.state === 'completed') {
			return $_('maintenance.detail.ended_at', { values: { rel: relativeTime(w.ends_at) } });
		}
		return $_('maintenance.detail.cancelled_at', { values: { rel: relativeTime(w.updated_at) } });
	}

	function ask(action: 'cancel' | 'end' | 'delete') {
		confirmAction = action;
		confirmOpen = true;
	}

	async function confirm() {
		if (!confirmAction || !detail) return;
		confirmBusy = true;
		try {
			if (confirmAction === 'cancel') {
				await cancelMaintenance(id);
				toastSuccess($_('maintenance.cancel.success'));
				await refresh();
			} else if (confirmAction === 'end') {
				await endMaintenanceNow(id);
				toastSuccess($_('maintenance.end_now.success'));
				await refresh();
			} else if (confirmAction === 'delete') {
				await deleteMaintenance(id);
				toastSuccess($_('maintenance.delete.success'));
				void goto('/maintenance');
				return;
			}
			confirmOpen = false;
			confirmAction = null;
		} catch (err) {
			toastError(err, $_('common.error_generic'));
		} finally {
			confirmBusy = false;
		}
	}

	function confirmTitle(): string {
		if (confirmAction === 'cancel') return $_('maintenance.cancel.confirm_title');
		if (confirmAction === 'end') return $_('maintenance.end_now.confirm_title');
		if (confirmAction === 'delete') return $_('maintenance.delete.confirm_title');
		return '';
	}

	function confirmBody(): string {
		if (confirmAction === 'cancel') return $_('maintenance.cancel.confirm_description');
		if (confirmAction === 'end') return $_('maintenance.end_now.confirm_description');
		if (confirmAction === 'delete') return $_('maintenance.delete.confirm_description');
		return '';
	}

	function fmtAbs(s: string): string {
		return new Date(s).toLocaleString();
	}
</script>

<div class="p-6">
	<a href="/maintenance" class="mb-4 inline-block text-sm text-muted-foreground hover:text-foreground">
		← {$_('maintenance.detail.back')}
	</a>

	{#if loading}
		<Skeleton class="mb-6 h-8 w-1/2" />
		<Skeleton class="h-32 w-full" />
	{:else if loadError || !detail}
		<Alert variant="destructive">
			<AlertTitle>{$_('maintenance.detail.not_found_title')}</AlertTitle>
			<AlertDescription>{$_('maintenance.detail.not_found_description')}</AlertDescription>
		</Alert>
	{:else}
		{@const w = detail.window}
		{@const sc = stateColor(w.state)}
		<PageHeader title={w.title}>
			{#snippet actions()}
				<div class="flex gap-2">
					{#if w.state === 'scheduled' || w.state === 'in_progress'}
						<Button variant="secondary" onclick={() => goto(`/maintenance/${id}/edit`)}>
							{$_('common.edit')}
						</Button>
					{/if}
					{#if w.state === 'scheduled'}
						<Button variant="secondary" onclick={() => ask('cancel')}>
							{$_('maintenance.actions.cancel')}
						</Button>
						<Button variant="destructive" onclick={() => ask('delete')}>
							{$_('common.delete')}
						</Button>
					{:else if w.state === 'in_progress'}
						<Button onclick={() => ask('end')}>{$_('maintenance.actions.end_now')}</Button>
					{:else if w.state === 'cancelled'}
						<Button variant="destructive" onclick={() => ask('delete')}>
							{$_('common.delete')}
						</Button>
					{/if}
				</div>
			{/snippet}
		</PageHeader>

		<div class="mb-6 flex items-center gap-3">
			<Badge
				variant="outline"
				style="color: {sc}; border-color: {sc};"
				class={w.state === 'cancelled' ? 'line-through' : ''}
			>
				{$_(`maintenance.state.${w.state}`)}
			</Badge>
			<span class="text-sm text-muted-foreground">{relativeWindow(w)}</span>
		</div>

		<div class="grid gap-6 lg:grid-cols-3">
			<div class="space-y-6 lg:col-span-2">
				<section class="rounded-md border border-border p-4">
					<h2 class="mb-3 text-sm font-medium text-muted-foreground">
						{$_('maintenance.detail.schedule_title')}
					</h2>
					<dl class="grid grid-cols-3 gap-2 text-sm">
						<dt class="text-muted-foreground">{$_('maintenance.fields.starts_at')}</dt>
						<dd class="col-span-2 text-foreground">{fmtAbs(w.starts_at)}</dd>
						<dt class="text-muted-foreground">{$_('maintenance.fields.ends_at')}</dt>
						<dd class="col-span-2 text-foreground">{fmtAbs(w.ends_at)}</dd>
						<dt class="text-muted-foreground">{$_('maintenance.detail.duration')}</dt>
						<dd class="col-span-2 text-foreground">{durationText(w.starts_at, w.ends_at)}</dd>
					</dl>
				</section>

				{#if w.description}
					<section class="rounded-md border border-border p-4">
						<h2 class="mb-3 text-sm font-medium text-muted-foreground">
							{$_('maintenance.fields.description')}
						</h2>
						<div class="prose prose-sm max-w-none text-foreground">
							{@html w.description_html}
						</div>
					</section>
				{/if}
			</div>

			<aside>
				<section class="rounded-md border border-border p-4">
					<h2 class="mb-3 text-sm font-medium text-muted-foreground">
						{$_('maintenance.fields.affected_components')}
					</h2>
					{#if detail.affected_components.length === 0}
						<p class="text-sm text-muted-foreground">{$_('common.na')}</p>
					{:else}
						<ul class="space-y-1.5">
							{#each detail.affected_components as c (c.id)}
								<li>
									<a
										href={`/components/${c.id}`}
										class="block rounded-md px-2 py-1.5 text-sm text-foreground hover:bg-accent"
									>
										{c.name}
									</a>
								</li>
							{/each}
						</ul>
					{/if}
				</section>
			</aside>
		</div>
	{/if}
</div>

<Dialog open={confirmOpen} onOpenChange={(v) => (confirmOpen = v)}>
	<DialogContent class="sm:max-w-[460px]">
		<DialogHeader>
			<DialogTitle>{confirmTitle()}</DialogTitle>
			<DialogDescription>{confirmBody()}</DialogDescription>
		</DialogHeader>
		<DialogFooter>
			<Button variant="secondary" onclick={() => (confirmOpen = false)}>
				{$_('common.cancel')}
			</Button>
			<Button variant="destructive" onclick={confirm} disabled={confirmBusy}>
				{confirmBusy ? $_('common.loading') : $_('common.confirm')}
			</Button>
		</DialogFooter>
	</DialogContent>
</Dialog>
