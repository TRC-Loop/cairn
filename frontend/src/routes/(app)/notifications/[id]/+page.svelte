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
	import NotificationChannelDialog from '$lib/components/notifications/NotificationChannelDialog.svelte';
	import {
		getChannel,
		deleteChannel,
		testChannel,
		listDeliveries,
		type NotificationChannel,
		type NotificationDelivery
	} from '$lib/api/notifications';
	import { ApiError } from '$lib/api/client';
	import { toastError, toastSuccess } from '$lib/toast';

	let channel = $state<NotificationChannel | null>(null);
	let deliveries = $state<NotificationDelivery[]>([]);
	let loading = $state(true);
	let notFound = $state(false);

	let editOpen = $state(false);
	let deleteOpen = $state(false);
	let deleting = $state(false);
	let testing = $state(false);

	const id = Number(page.params.id);
	let pollHandle: ReturnType<typeof setInterval> | null = null;

	async function load() {
		loading = true;
		notFound = false;
		try {
			const c = await getChannel(id);
			channel = c;
			await loadDeliveries();
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

	async function loadDeliveries() {
		try {
			deliveries = await listDeliveries(id);
		} catch {
			deliveries = [];
		}
	}

	onMount(() => {
		void load();
		pollHandle = setInterval(() => {
			void loadDeliveries();
		}, 10000);
		return () => {
			if (pollHandle) clearInterval(pollHandle);
		};
	});

	async function confirmDelete() {
		if (!channel) return;
		deleting = true;
		try {
			await deleteChannel(channel.id);
			toastSuccess($_('notifications.delete.success'));
			await goto('/notifications');
		} catch (err) {
			toastError(err, $_('common.error_generic'));
		} finally {
			deleting = false;
		}
	}

	async function sendTest() {
		if (!channel) return;
		testing = true;
		try {
			await testChannel(channel.id);
			toastSuccess($_('notifications.test.queued'));
			await loadDeliveries();
		} catch (err) {
			toastError(err, $_('common.error_generic'));
		} finally {
			testing = false;
		}
	}

	function statusColorClass(status: string) {
		switch (status) {
			case 'sent':
				return 'text-[color:var(--status-up)]';
			case 'failed':
				return 'text-[color:var(--status-down)]';
			case 'sending':
				return 'text-[color:var(--status-degraded)]';
			default:
				return 'text-muted-foreground';
		}
	}

	function fmt(ts: string | null) {
		if (!ts) return $_('common.na');
		return new Date(ts).toLocaleString();
	}
</script>

<div class="p-6">
	<a
		href="/notifications"
		class="mb-4 inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
	>
		<IconArrowLeft size={14} />
		{$_('notifications.detail.back')}
	</a>

	{#if loading}
		<Skeleton class="mb-6 h-10 w-64" />
		<Skeleton class="mb-2 h-40 w-full" />
	{:else if notFound}
		<Alert variant="destructive">
			<AlertTitle>{$_('notifications.detail.not_found_title')}</AlertTitle>
			<AlertDescription>{$_('notifications.detail.not_found_description')}</AlertDescription>
		</Alert>
		<Button href="/notifications" variant="outline" class="mt-4">
			{$_('notifications.detail.back')}
		</Button>
	{:else if channel}
		{@const c = channel}
		<PageHeader title={c.name} description={$_(`notifications.types.${c.type}`)}>
			{#snippet actions()}
				<Button variant="secondary" onclick={sendTest} disabled={testing || !c.enabled}>
					{testing ? $_('notifications.test.sending') : $_('notifications.actions.send_test')}
				</Button>
				<Button variant="secondary" onclick={() => (editOpen = true)}>
					{$_('common.edit')}
				</Button>
				<Button variant="destructive" onclick={() => (deleteOpen = true)}>
					{$_('common.delete')}
				</Button>
			{/snippet}
		</PageHeader>

		<div class="mb-6 grid grid-cols-2 gap-4 rounded-md border border-border p-4 text-sm md:grid-cols-4">
			<div>
				<div class="text-xs uppercase tracking-wide text-muted-foreground">
					{$_('notifications.detail.status')}
				</div>
				<div class="text-foreground">
					{c.enabled
						? $_('notifications.status.enabled')
						: $_('notifications.status.disabled')}
				</div>
			</div>
			<div>
				<div class="text-xs uppercase tracking-wide text-muted-foreground">
					{$_('notifications.detail.used_by')}
				</div>
				<div class="text-foreground">
					{c.used_by_check_count}
				</div>
			</div>
			<div>
				<div class="text-xs uppercase tracking-wide text-muted-foreground">
					{$_('notifications.fields.retry_max')}
				</div>
				<div class="text-foreground">{c.retry_max}</div>
			</div>
			<div>
				<div class="text-xs uppercase tracking-wide text-muted-foreground">
					{$_('notifications.fields.retry_backoff_seconds')}
				</div>
				<div class="text-foreground">{c.retry_backoff_seconds}s</div>
			</div>
		</div>

		<div class="rounded-md border border-border p-4">
			<h2 class="mb-3 text-sm font-medium text-foreground">
				{$_('notifications.detail.recent_deliveries')}
			</h2>
			{#if deliveries.length === 0}
				<p class="text-sm text-muted-foreground">{$_('notifications.detail.no_deliveries')}</p>
			{:else}
				<table class="w-full text-sm">
					<thead>
						<tr class="border-b border-border text-left text-xs uppercase tracking-wide text-muted-foreground">
							<th class="py-1.5 pr-3">{$_('notifications.detail.col_event')}</th>
							<th class="py-1.5 pr-3">{$_('notifications.detail.col_status')}</th>
							<th class="py-1.5 pr-3">{$_('notifications.detail.col_attempts')}</th>
							<th class="py-1.5 pr-3">{$_('notifications.detail.col_last_attempt')}</th>
							<th class="py-1.5 pr-3">{$_('notifications.detail.col_error')}</th>
						</tr>
					</thead>
					<tbody>
						{#each deliveries as d (d.id)}
							<tr class="border-b border-border last:border-b-0">
								<td class="py-1.5 pr-3 text-foreground">{d.event_type}</td>
								<td class="py-1.5 pr-3 {statusColorClass(d.status)}">{d.status}</td>
								<td class="py-1.5 pr-3 text-muted-foreground">{d.attempt_count}</td>
								<td class="py-1.5 pr-3 text-muted-foreground">{fmt(d.last_attempted_at)}</td>
								<td class="py-1.5 pr-3 text-muted-foreground">
									{d.last_error ?? ''}
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			{/if}
		</div>

		<NotificationChannelDialog
			open={editOpen}
			mode="edit"
			existing={channel}
			onOpenChange={(v) => (editOpen = v)}
			onSaved={load}
		/>

		<Dialog open={deleteOpen} onOpenChange={(v) => (deleteOpen = v)}>
			<DialogContent class="sm:max-w-[420px]">
				<DialogHeader>
					<DialogTitle>{$_('notifications.delete.confirm_title')}</DialogTitle>
					<DialogDescription>
						{$_('notifications.delete.confirm_description', { values: { name: channel.name } })}
					</DialogDescription>
				</DialogHeader>
				<DialogFooter>
					<Button variant="secondary" onclick={() => (deleteOpen = false)}>
						{$_('common.cancel')}
					</Button>
					<Button variant="destructive" onclick={confirmDelete} disabled={deleting}>
						{deleting ? $_('common.deleting') : $_('notifications.delete.confirm_button')}
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	{/if}
</div>
