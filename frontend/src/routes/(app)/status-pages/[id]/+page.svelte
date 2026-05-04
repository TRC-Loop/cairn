<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<script lang="ts">
	import { onMount } from 'svelte';
	import { _ } from 'svelte-i18n';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
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
	import { IconArrowLeft, IconArrowUp, IconArrowDown, IconTrash } from '@tabler/icons-svelte';
	import PageHeader from '$lib/components/common/PageHeader.svelte';
	import StatusPageDialog from '$lib/components/status-pages/StatusPageDialog.svelte';
	import FooterEditor from '$lib/components/status-pages/FooterEditor.svelte';
	import {
		getStatusPage,
		deleteStatusPage,
		setDefaultStatusPage,
		setStatusPagePassword,
		setStatusPageComponents,
		type StatusPage
	} from '$lib/api/statusPages';
	import { listComponents, type Component } from '$lib/api/components';
	import { ApiError } from '$lib/api/client';
	import { toastError, toastSuccess } from '$lib/toast';

	let statusPage = $state<StatusPage | null>(null);
	let linked = $state<Component[]>([]);
	let loading = $state(true);
	let notFound = $state(false);

	let editOpen = $state(false);
	let deleteOpen = $state(false);
	let deleting = $state(false);

	let passwordOpen = $state(false);
	let passwordValue = $state('');
	let passwordError = $state<string | null>(null);
	let savingPassword = $state(false);

	let addCompOpen = $state(false);
	let allComponents = $state<Component[]>([]);
	let selectedToAdd = $state<Set<number>>(new Set());

	const id = Number(page.params.id);

	async function load() {
		loading = true;
		notFound = false;
		try {
			const res = await getStatusPage(id);
			statusPage = res.status_page;
			linked = res.components;
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

	async function makeDefault() {
		if (!statusPage) return;
		try {
			await setDefaultStatusPage(statusPage.id);
			toastSuccess($_('status_pages.edit.success'));
			await load();
		} catch (err) {
			toastError(err, $_('common.error_generic'));
		}
	}

	async function confirmDelete() {
		if (!statusPage) return;
		deleting = true;
		try {
			await deleteStatusPage(statusPage.id);
			toastSuccess($_('status_pages.delete.success'));
			await goto('/status-pages');
		} catch (err) {
			toastError(err, $_('common.error_generic'));
		} finally {
			deleting = false;
		}
	}

	function openPassword() {
		passwordValue = '';
		passwordError = null;
		passwordOpen = true;
	}

	async function submitPassword(clear: boolean) {
		if (!statusPage) return;
		const value = clear ? '' : passwordValue;
		if (!clear && (value.length < 8 || value.length > 512)) {
			passwordError = $_('status_pages.detail.password_dialog_help');
			return;
		}
		savingPassword = true;
		try {
			await setStatusPagePassword(statusPage.id, value);
			toastSuccess($_('status_pages.edit.success'));
			passwordOpen = false;
			await load();
		} catch (err) {
			if (err instanceof ApiError && err.fields?.password) {
				passwordError = err.fields.password;
			} else {
				toastError(err, $_('common.error_generic'));
			}
		} finally {
			savingPassword = false;
		}
	}

	async function openAddComponents() {
		try {
			allComponents = await listComponents();
			selectedToAdd = new Set();
			addCompOpen = true;
		} catch (err) {
			toastError(err, $_('common.error_generic'));
		}
	}

	const availableToAdd = $derived(
		allComponents.filter((c) => !linked.some((l) => l.id === c.id))
	);

	function toggleSelect(cid: number) {
		const next = new Set(selectedToAdd);
		if (next.has(cid)) next.delete(cid);
		else next.add(cid);
		selectedToAdd = next;
	}

	async function commitAddComponents() {
		if (!statusPage || selectedToAdd.size === 0) return;
		const newOrder = [...linked.map((c) => c.id), ...selectedToAdd];
		try {
			await setStatusPageComponents(statusPage.id, newOrder);
			toastSuccess($_('status_pages.edit.success'));
			addCompOpen = false;
			await load();
		} catch (err) {
			toastError(err, $_('common.error_generic'));
		}
	}

	async function persistOrder(newLinked: Component[]) {
		if (!statusPage) return;
		linked = newLinked;
		try {
			await setStatusPageComponents(
				statusPage.id,
				newLinked.map((c) => c.id)
			);
		} catch (err) {
			toastError(err, $_('common.error_generic'));
			await load();
		}
	}

	async function moveUp(idx: number) {
		if (idx <= 0) return;
		const next = [...linked];
		[next[idx - 1], next[idx]] = [next[idx], next[idx - 1]];
		await persistOrder(next);
	}

	async function moveDown(idx: number) {
		if (idx >= linked.length - 1) return;
		const next = [...linked];
		[next[idx], next[idx + 1]] = [next[idx + 1], next[idx]];
		await persistOrder(next);
	}

	async function removeComponent(cid: number) {
		const next = linked.filter((c) => c.id !== cid);
		await persistOrder(next);
	}
</script>

<div class="p-6">
	<a
		href="/status-pages"
		class="mb-4 inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
	>
		<IconArrowLeft size={14} />
		{$_('status_pages.detail.back')}
	</a>

	{#if loading}
		<Skeleton class="mb-6 h-10 w-64" />
		<Skeleton class="mb-2 h-40 w-full" />
	{:else if notFound}
		<Alert variant="destructive">
			<AlertTitle>{$_('status_pages.detail.not_found_title')}</AlertTitle>
			<AlertDescription>{$_('status_pages.detail.not_found_description')}</AlertDescription>
		</Alert>
		<Button href="/status-pages" variant="outline" class="mt-4">
			{$_('status_pages.detail.back')}
		</Button>
	{:else if statusPage}
		{@const sp = statusPage}
		<PageHeader title={sp.title} description={`/p/${sp.slug}`}>
			{#snippet actions()}
				{#if sp.is_default}
					<span class="inline-flex items-center rounded-md border border-border px-2 py-1 text-xs text-muted-foreground">
						{$_('status_pages.list.default_badge')}
					</span>
				{:else}
					<Button variant="secondary" onclick={makeDefault}>
						{$_('status_pages.detail.set_default')}
					</Button>
				{/if}
				<Button
					variant="secondary"
					onclick={() => window.open(`/p/${sp.slug}`, '_blank')}
				>
					{$_('status_pages.detail.view_public')}
				</Button>
				<Button variant="secondary" onclick={() => (editOpen = true)}>
					{$_('status_pages.detail.edit')}
				</Button>
				<Button variant="destructive" onclick={() => (deleteOpen = true)}>
					{$_('status_pages.detail.delete')}
				</Button>
			{/snippet}
		</PageHeader>

		<div class="mb-6 rounded-md border border-border p-4">
			<h2 class="mb-3 text-sm font-medium text-foreground">
				{$_('status_pages.detail.password_section')}
			</h2>
			<p class="mb-3 text-sm text-muted-foreground">
				{sp.password_set
					? $_('status_pages.detail.password_set')
					: $_('status_pages.detail.password_unset')}
			</p>
			<div class="flex gap-2">
				<Button variant="secondary" size="sm" onclick={openPassword}>
					{sp.password_set
						? $_('status_pages.detail.password_change')
						: $_('status_pages.detail.password_add')}
				</Button>
				{#if sp.password_set}
					<Button variant="secondary" size="sm" onclick={() => submitPassword(true)}>
						{$_('status_pages.detail.password_remove')}
					</Button>
				{/if}
			</div>
		</div>

		<div class="rounded-md border border-border p-4">
			<div class="mb-3 flex items-center justify-between">
				<h2 class="text-sm font-medium text-foreground">
					{$_('status_pages.detail.components_section')}
				</h2>
				<Button size="sm" variant="secondary" onclick={openAddComponents}>
					{$_('status_pages.detail.add_component')}
				</Button>
			</div>
			{#if linked.length === 0}
				<p class="text-sm text-muted-foreground">
					{$_('status_pages.detail.components_empty')}
				</p>
			{:else}
				<ul class="divide-y divide-border">
					{#each linked as c, idx (c.id)}
						<li class="flex items-center justify-between py-2 text-sm">
							<a href={`/components/${c.id}`} class="text-foreground hover:underline">
								{c.name}
							</a>
							<div class="flex items-center gap-1">
								<button
									type="button"
									aria-label={$_('status_pages.detail.move_up')}
									class="inline-flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground hover:bg-accent hover:text-foreground disabled:opacity-50"
									disabled={idx === 0}
									onclick={() => moveUp(idx)}
								>
									<IconArrowUp size={14} />
								</button>
								<button
									type="button"
									aria-label={$_('status_pages.detail.move_down')}
									class="inline-flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground hover:bg-accent hover:text-foreground disabled:opacity-50"
									disabled={idx === linked.length - 1}
									onclick={() => moveDown(idx)}
								>
									<IconArrowDown size={14} />
								</button>
								<button
									type="button"
									aria-label={$_('status_pages.detail.remove_component')}
									class="inline-flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground hover:bg-accent hover:text-destructive"
									onclick={() => removeComponent(c.id)}
								>
									<IconTrash size={14} />
								</button>
							</div>
						</li>
					{/each}
				</ul>
			{/if}
		</div>

		<div class="mt-6">
			<FooterEditor statusPageId={sp.id} />
		</div>

		<StatusPageDialog
			open={editOpen}
			mode="edit"
			existing={statusPage}
			onOpenChange={(v) => (editOpen = v)}
			onSaved={load}
		/>

		<Dialog open={deleteOpen} onOpenChange={(v) => (deleteOpen = v)}>
			<DialogContent class="sm:max-w-[420px]">
				<DialogHeader>
					<DialogTitle>{$_('status_pages.delete.confirm_title')}</DialogTitle>
					<DialogDescription>
						{$_('status_pages.delete.confirm_description', { values: { title: sp.title } })}
					</DialogDescription>
				</DialogHeader>
				<DialogFooter>
					<Button variant="secondary" onclick={() => (deleteOpen = false)}>
						{$_('common.cancel')}
					</Button>
					<Button variant="destructive" onclick={confirmDelete} disabled={deleting}>
						{deleting ? $_('common.deleting') : $_('status_pages.delete.confirm_button')}
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>

		<Dialog open={passwordOpen} onOpenChange={(v) => (passwordOpen = v)}>
			<DialogContent class="sm:max-w-[420px]">
				<DialogHeader>
					<DialogTitle>{$_('status_pages.detail.password_dialog_title')}</DialogTitle>
					<DialogDescription>
						{$_('status_pages.detail.password_dialog_help')}
					</DialogDescription>
				</DialogHeader>
				<div class="space-y-1.5 py-2">
					<Label for="sp-password">{$_('status_pages.detail.password_field')}</Label>
					<Input
						id="sp-password"
						type="password"
						bind:value={passwordValue}
						maxlength={512}
					/>
					{#if passwordError}
						<p class="text-xs text-destructive">{passwordError}</p>
					{/if}
				</div>
				<DialogFooter>
					<Button
						variant="secondary"
						onclick={() => (passwordOpen = false)}
						disabled={savingPassword}
					>
						{$_('common.cancel')}
					</Button>
					<Button onclick={() => submitPassword(false)} disabled={savingPassword}>
						{savingPassword ? $_('common.saving') : $_('common.save')}
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>

		<Dialog open={addCompOpen} onOpenChange={(v) => (addCompOpen = v)}>
			<DialogContent class="sm:max-w-[460px]">
				<DialogHeader>
					<DialogTitle>{$_('status_pages.detail.add_component_dialog_title')}</DialogTitle>
				</DialogHeader>
				<div class="max-h-[50vh] overflow-y-auto py-2">
					{#if availableToAdd.length === 0}
						<p class="text-sm text-muted-foreground">
							{$_('status_pages.detail.add_component_none')}
						</p>
					{:else}
						<ul class="space-y-1">
							{#each availableToAdd as c (c.id)}
								<li>
									<label class="flex items-center gap-2 rounded-md px-2 py-1.5 text-sm hover:bg-accent">
										<input
											type="checkbox"
											checked={selectedToAdd.has(c.id)}
											onchange={() => toggleSelect(c.id)}
											class="rounded border-input"
										/>
										<span class="text-foreground">{c.name}</span>
									</label>
								</li>
							{/each}
						</ul>
					{/if}
				</div>
				<DialogFooter>
					<Button variant="secondary" onclick={() => (addCompOpen = false)}>
						{$_('common.cancel')}
					</Button>
					<Button onclick={commitAddComponents} disabled={selectedToAdd.size === 0}>
						{$_('common.save')}
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	{/if}
</div>
