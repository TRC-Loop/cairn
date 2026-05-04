<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<script lang="ts">
	import { onMount } from 'svelte';
	import { _ } from 'svelte-i18n';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Alert, AlertTitle, AlertDescription } from '$lib/components/ui/alert';
	import { Badge } from '$lib/components/ui/badge';
	import {
		Dialog,
		DialogContent,
		DialogHeader,
		DialogTitle,
		DialogDescription,
		DialogFooter
	} from '$lib/components/ui/dialog';
	import * as Select from '$lib/components/ui/select';
	import { auth } from '$lib/stores/auth';
	import {
		getIncident,
		addIncidentUpdate,
		addAffectedCheck,
		removeAffectedCheck,
		patchIncident,
		deleteIncident,
		type IncidentDetail,
		type IncidentStatus,
		type IncidentSeverity
	} from '$lib/api/incidents';
	import { listMonitors, type Monitor } from '$lib/api/monitors';
	import { relativeTime } from '$lib/utils';
	import { toastError, toastSuccess } from '$lib/toast';
	import { apiRequest } from '$lib/api/client';

	const incidentId = $derived(Number(page.params.id));

	let detail = $state<IncidentDetail | null>(null);
	let loading = $state(true);
	let notFound = $state(false);
	let loadError = $state<string | null>(null);

	let role = $state<'admin' | 'editor' | 'viewer' | null>(null);
	const canEdit = $derived(role === 'admin' || role === 'editor');

	const unsub = auth.subscribe((s) => {
		role = s.user?.role ?? null;
	});
	$effect(() => () => unsub());

	let updateMessage = $state('');
	let updateStatus = $state<string>('keep');
	let posting = $state(false);
	let composeTab = $state<'edit' | 'preview'>('edit');
	let previewHtml = $state('');
	let previewLoading = $state(false);

	async function loadPreview() {
		const msg = updateMessage.trim();
		if (!msg) {
			previewHtml = '';
			return;
		}
		previewLoading = true;
		try {
			const res = await apiRequest<{ html: string }>('/api/preview-markdown', {
				method: 'POST',
				body: { markdown: updateMessage }
			});
			previewHtml = res.html;
		} catch (err) {
			toastError(err);
			previewHtml = '';
		} finally {
			previewLoading = false;
		}
	}

	function selectTab(tab: 'edit' | 'preview') {
		composeTab = tab;
		if (tab === 'preview') void loadPreview();
	}

	let resolveOpen = $state(false);
	let resolving = $state(false);

	let deleteOpen = $state(false);
	let deleting = $state(false);

	let addOpen = $state(false);
	let availableMonitors = $state<Monitor[]>([]);
	let pickMonitorId = $state<string>('');
	let adding = $state(false);

	async function refresh() {
		loading = true;
		notFound = false;
		loadError = null;
		try {
			detail = await getIncident(incidentId);
		} catch (err) {
			const e = err as { status?: number };
			if (e?.status === 404) {
				notFound = true;
			} else {
				loadError = $_('common.error_generic');
				toastError(err, $_('common.error_generic'));
			}
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		void refresh();
	});

	function severityColor(s: IncidentSeverity): string {
		switch (s) {
			case 'minor':
				return 'var(--status-degraded)';
			case 'major':
			case 'critical':
				return 'var(--status-down)';
		}
	}

	function statusColor(s: string): string {
		switch (s) {
			case 'resolved':
				return 'var(--status-up)';
			case 'monitoring':
				return 'var(--status-degraded)';
			default:
				return 'var(--status-down)';
		}
	}

	const allStatuses: IncidentStatus[] = ['investigating', 'identified', 'monitoring', 'resolved'];

	async function postUpdate(e: Event) {
		e.preventDefault();
		if (!detail) return;
		const msg = updateMessage.trim();
		if (!msg) {
			toastError(null, $_('incidents.create.message_required'));
			return;
		}
		posting = true;
		try {
			const newStatus =
				updateStatus === 'keep' ? undefined : (updateStatus as IncidentStatus);
			await addIncidentUpdate(detail.incident.id, {
				message: msg,
				new_status: newStatus
			});
			toastSuccess($_('incidents.detail.add_update_success'));
			updateMessage = '';
			updateStatus = 'keep';
			await refresh();
		} catch (err) {
			toastError(err, $_('common.error_generic'));
		} finally {
			posting = false;
		}
	}

	async function confirmResolve() {
		if (!detail) return;
		resolving = true;
		try {
			await addIncidentUpdate(detail.incident.id, {
				message: 'Resolved.',
				new_status: 'resolved'
			});
			toastSuccess($_('incidents.detail.add_update_success'));
			resolveOpen = false;
			await refresh();
		} catch (err) {
			toastError(err, $_('common.error_generic'));
		} finally {
			resolving = false;
		}
	}

	async function confirmDelete() {
		if (!detail) return;
		deleting = true;
		try {
			await deleteIncident(detail.incident.id);
			toastSuccess($_('incidents.delete.success'));
			void goto('/incidents');
		} catch (err) {
			toastError(err, $_('common.error_generic'));
		} finally {
			deleting = false;
		}
	}

	async function openAddAffected() {
		try {
			availableMonitors = await listMonitors();
			pickMonitorId = '';
			addOpen = true;
		} catch (err) {
			toastError(err, $_('common.error_generic'));
		}
	}

	async function commitAddAffected() {
		if (!detail || !pickMonitorId) return;
		adding = true;
		try {
			await addAffectedCheck(detail.incident.id, Number(pickMonitorId));
			addOpen = false;
			await refresh();
		} catch (err) {
			toastError(err, $_('common.error_generic'));
		} finally {
			adding = false;
		}
	}

	async function unlinkAffected(checkId: number) {
		if (!detail) return;
		try {
			await removeAffectedCheck(detail.incident.id, checkId);
			await refresh();
		} catch (err) {
			toastError(err, $_('common.error_generic'));
		}
	}

	const linkedIds = $derived(new Set(detail?.affected_checks.map((c) => c.id) ?? []));
	const eligibleMonitors = $derived(availableMonitors.filter((m) => !linkedIds.has(m.id)));
</script>

<div class="p-6">
	<a href="/incidents" class="mb-4 inline-block text-sm text-muted-foreground hover:text-foreground">
		← {$_('incidents.detail.back')}
	</a>

	{#if loading && !detail}
		<div class="space-y-3">
			<Skeleton class="h-8 w-1/2" />
			<Skeleton class="h-4 w-1/3" />
			<Skeleton class="h-32 w-full" />
		</div>
	{:else if notFound}
		<Alert>
			<AlertTitle>{$_('incidents.detail.not_found_title')}</AlertTitle>
			<AlertDescription>{$_('incidents.detail.not_found_description')}</AlertDescription>
		</Alert>
	{:else if loadError}
		<Alert variant="destructive">
			<AlertTitle>{$_('common.error_generic')}</AlertTitle>
			<AlertDescription>{loadError}</AlertDescription>
		</Alert>
		<Button variant="outline" class="mt-4" onclick={refresh}>{$_('common.retry')}</Button>
	{:else if detail}
		{@const inc = detail.incident}
		{@const sevColor = severityColor(inc.severity)}
		{@const stColor = statusColor(inc.status)}

		<div class="mb-6 flex items-start justify-between gap-4">
			<div class="min-w-0 flex-1">
				<div class="mb-2 flex flex-wrap items-center gap-2">
					<Badge variant="outline" style="color: {sevColor}; border-color: {sevColor};">
						{$_(`incidents.severity.${inc.severity}`)}
					</Badge>
					<Badge variant="outline" style="color: {stColor}; border-color: {stColor};">
						{$_(`incidents.status.${inc.status}`)}
					</Badge>
					{#if inc.auto_created}
						<Badge variant="outline" class="text-muted-foreground">
							{$_('incidents.source.auto')}
						</Badge>
					{/if}
				</div>
				<div class="flex items-baseline gap-2">
					<span class="font-mono text-sm text-muted-foreground">{inc.display_id}</span>
					<h1 class="text-xl font-medium tracking-tight text-foreground">{inc.title}</h1>
				</div>
				<p class="mt-1 text-sm text-muted-foreground">
					{$_('incidents.list.col_started')}: {relativeTime(inc.started_at)}
					{#if inc.resolved_at}
						· {$_('incidents.list.col_resolved')}: {relativeTime(inc.resolved_at)}
					{/if}
				</p>
				{#if inc.auto_created && inc.triggering_check}
					<p class="mt-1 text-sm text-muted-foreground">
						{$_('incidents.detail.triggering_check')}:
						<a
							href={`/monitors/${inc.triggering_check.id}`}
							class="text-foreground hover:underline"
						>
							{inc.triggering_check.name}
						</a>
					</p>
				{/if}
			</div>
			{#if canEdit}
				<div class="flex shrink-0 gap-2">
					<Button variant="outline" onclick={() => goto(`/incidents/${inc.id}/edit`)}>
						{$_('common.edit')}
					</Button>
					{#if inc.status !== 'resolved'}
						<Button onclick={() => (resolveOpen = true)}>
							{$_('incidents.detail.resolve')}
						</Button>
					{/if}
					<Button variant="destructive" onclick={() => (deleteOpen = true)}>
						{$_('common.delete')}
					</Button>
				</div>
			{/if}
		</div>

		<div class="grid gap-6 lg:grid-cols-[1fr_320px]">
			<div class="space-y-6">
				<section>
					<h2 class="mb-3 text-sm font-medium tracking-tight text-foreground">
						{$_('incidents.detail.timeline_title')}
					</h2>
					{#if detail.updates.length === 0}
						<p class="text-sm text-muted-foreground">{$_('incidents.detail.timeline_empty')}</p>
					{:else}
						<ol class="space-y-4">
							{#each detail.updates as u, idx (u.id)}
								{@const uColor = statusColor(u.status)}
								{@const isFirst = idx === 0}
								{@const isLast = idx === detail.updates.length - 1}
								<li class="grid grid-cols-[16px_1fr] items-start gap-3">
									<div class="relative h-full min-h-[20px]">
										{#if !isFirst}
											<span
												class="absolute left-1/2 top-0 h-[10px] w-px -translate-x-1/2 bg-border"
												aria-hidden="true"
											></span>
										{/if}
										{#if !isLast}
											<span
												class="absolute left-1/2 top-[10px] bottom-0 w-px -translate-x-1/2 bg-border"
												aria-hidden="true"
											></span>
										{/if}
										<span
											class="absolute left-1/2 top-[10px] h-2 w-2 -translate-x-1/2 -translate-y-1/2 rounded-full"
											style="background-color: {uColor};"
											aria-hidden="true"
										></span>
									</div>
									<div class="min-w-0">
										<div class="flex flex-wrap items-center gap-2">
											<Badge variant="outline" style="color: {uColor}; border-color: {uColor};">
												{$_(`incidents.status.${u.status}`)}
											</Badge>
											<span class="text-xs text-muted-foreground">
												{relativeTime(u.created_at)}
											</span>
											{#if u.auto_generated}
												<span class="text-xs text-muted-foreground"
													>· {$_('incidents.source.auto')}</span
												>
											{/if}
										</div>
										<div class="markdown-body mt-1 text-sm text-foreground">
											{@html u.message_html || u.message}
										</div>
									</div>
								</li>
							{/each}
						</ol>
					{/if}
				</section>

				{#if canEdit && inc.status !== 'resolved'}
					<section class="rounded-md border border-border p-4">
						<h2 class="mb-3 text-sm font-medium tracking-tight text-foreground">
							{$_('incidents.detail.add_update')}
						</h2>
						<form class="space-y-3" onsubmit={postUpdate}>
							<div class="space-y-1.5">
								<Label for="i-message">{$_('incidents.fields.message')}</Label>
								<div class="flex gap-1 border-b border-border">
									<button
										type="button"
										class="border-b-2 px-3 py-1.5 text-xs transition-colors {composeTab ===
										'edit'
											? 'border-primary text-foreground'
											: 'border-transparent text-muted-foreground hover:text-foreground'}"
										onclick={() => selectTab('edit')}
									>
										{$_('common.edit')}
									</button>
									<button
										type="button"
										class="border-b-2 px-3 py-1.5 text-xs transition-colors {composeTab ===
										'preview'
											? 'border-primary text-foreground'
											: 'border-transparent text-muted-foreground hover:text-foreground'}"
										onclick={() => selectTab('preview')}
									>
										{$_('common.preview')}
									</button>
								</div>
								{#if composeTab === 'edit'}
									<textarea
										id="i-message"
										bind:value={updateMessage}
										rows="4"
										maxlength={2000}
										class="flex w-full rounded-md border border-input bg-transparent px-3 py-2 font-mono text-sm shadow-xs placeholder:text-muted-foreground focus-visible:border-ring focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring/50"
									></textarea>
								{:else}
									<div
										class="markdown-body min-h-[6rem] rounded-md border border-input px-3 py-2 text-sm"
									>
										{#if previewLoading}
											<span class="text-muted-foreground">{$_('common.loading')}</span>
										{:else if previewHtml}
											{@html previewHtml}
										{:else}
											<span class="text-muted-foreground">{$_('common.no_results')}</span>
										{/if}
									</div>
								{/if}
								<p class="text-xs text-muted-foreground">{$_('common.markdown_supported')}</p>
							</div>
							<div class="space-y-1.5">
								<Label for="i-status">{$_('incidents.fields.new_status')}</Label>
								<Select.Root type="single" bind:value={updateStatus}>
									<Select.Trigger id="i-status" class="w-full">
										{updateStatus === 'keep'
											? $_('incidents.fields.keep_status')
											: $_(`incidents.status.${updateStatus}`)}
									</Select.Trigger>
									<Select.Content>
										<Select.Item value="keep">{$_('incidents.fields.keep_status')}</Select.Item>
										{#each allStatuses as s (s)}
											<Select.Item value={s}>{$_(`incidents.status.${s}`)}</Select.Item>
										{/each}
									</Select.Content>
								</Select.Root>
							</div>
							<Button type="submit" disabled={posting || !updateMessage.trim()}>
								{posting ? $_('common.saving') : $_('incidents.detail.add_update_submit')}
							</Button>
						</form>
					</section>
				{/if}
			</div>

			<aside>
				<section>
					<div class="mb-3 flex items-center justify-between">
						<h2 class="text-sm font-medium tracking-tight text-foreground">
							{$_('incidents.detail.affected_title')}
						</h2>
						{#if canEdit}
							<Button variant="outline" size="sm" onclick={openAddAffected}>
								{$_('incidents.detail.add_affected')}
							</Button>
						{/if}
					</div>
					{#if detail.affected_checks.length === 0}
						<p class="text-sm text-muted-foreground">
							{$_('incidents.detail.affected_empty')}
						</p>
					{:else}
						<ul class="space-y-1.5">
							{#each detail.affected_checks as m (m.id)}
								<li
									class="flex items-center justify-between gap-2 rounded-md border border-border px-3 py-1.5 text-sm"
								>
									<a href={`/monitors/${m.id}`} class="min-w-0 flex-1 truncate text-foreground hover:underline">
										{m.name}
									</a>
									{#if canEdit}
										<button
											type="button"
											class="text-xs text-muted-foreground hover:text-foreground"
											onclick={() => unlinkAffected(m.id)}
											aria-label={$_('incidents.detail.remove_affected')}
										>
											×
										</button>
									{/if}
								</li>
							{/each}
						</ul>
					{/if}
				</section>
			</aside>
		</div>
	{/if}
</div>

<Dialog open={resolveOpen} onOpenChange={(v) => (resolveOpen = v)}>
	<DialogContent class="sm:max-w-[420px]">
		<DialogHeader>
			<DialogTitle>{$_('incidents.detail.resolve_confirm_title')}</DialogTitle>
			<DialogDescription>
				{$_('incidents.detail.resolve_confirm_description')}
			</DialogDescription>
		</DialogHeader>
		<DialogFooter>
			<Button variant="secondary" onclick={() => (resolveOpen = false)}>
				{$_('common.cancel')}
			</Button>
			<Button onclick={confirmResolve} disabled={resolving}>
				{resolving ? $_('common.saving') : $_('incidents.detail.resolve')}
			</Button>
		</DialogFooter>
	</DialogContent>
</Dialog>

<Dialog open={deleteOpen} onOpenChange={(v) => (deleteOpen = v)}>
	<DialogContent class="sm:max-w-[420px]">
		<DialogHeader>
			<DialogTitle>{$_('incidents.delete.confirm_title')}</DialogTitle>
			<DialogDescription>
				{#if detail}
					{$_('incidents.delete.confirm_description', { values: { title: detail.incident.title } })}
				{/if}
			</DialogDescription>
		</DialogHeader>
		<DialogFooter>
			<Button variant="secondary" onclick={() => (deleteOpen = false)}>
				{$_('common.cancel')}
			</Button>
			<Button variant="destructive" onclick={confirmDelete} disabled={deleting}>
				{deleting ? $_('common.deleting') : $_('incidents.delete.confirm_button')}
			</Button>
		</DialogFooter>
	</DialogContent>
</Dialog>

<Dialog open={addOpen} onOpenChange={(v) => (addOpen = v)}>
	<DialogContent class="sm:max-w-[420px]">
		<DialogHeader>
			<DialogTitle>{$_('incidents.detail.add_affected')}</DialogTitle>
		</DialogHeader>
		<div class="space-y-3 py-2">
			{#if eligibleMonitors.length === 0}
				<p class="text-sm text-muted-foreground">
					{$_('components.attach_monitors.empty')}
				</p>
			{:else}
				<Select.Root type="single" bind:value={pickMonitorId}>
					<Select.Trigger class="w-full">
						{pickMonitorId
							? eligibleMonitors.find((m) => String(m.id) === pickMonitorId)?.name
							: $_('common.na')}
					</Select.Trigger>
					<Select.Content>
						{#each eligibleMonitors as m (m.id)}
							<Select.Item value={String(m.id)}>{m.name}</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
			{/if}
		</div>
		<DialogFooter>
			<Button variant="secondary" onclick={() => (addOpen = false)} disabled={adding}>
				{$_('common.cancel')}
			</Button>
			<Button onclick={commitAddAffected} disabled={!pickMonitorId || adding}>
				{adding ? $_('common.saving') : $_('common.save')}
			</Button>
		</DialogFooter>
	</DialogContent>
</Dialog>
