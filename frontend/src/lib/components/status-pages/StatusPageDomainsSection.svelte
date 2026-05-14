<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<script lang="ts">
	import { onMount } from 'svelte';
	import { _ } from 'svelte-i18n';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import {
		listStatusPageDomains,
		addStatusPageDomain,
		deleteStatusPageDomain,
		type StatusPageDomain
	} from '$lib/api/statusPages';
	import { ApiError } from '$lib/api/client';
	import { toastError, toastSuccess } from '$lib/toast';

	type Props = { statusPageId: number };
	let { statusPageId }: Props = $props();

	let domains = $state<StatusPageDomain[]>([]);
	let loading = $state(true);
	let newDomain = $state('');
	let saving = $state(false);
	let fieldError = $state('');

	const re = /^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*$/;

	async function load() {
		loading = true;
		try {
			domains = await listStatusPageDomains(statusPageId);
		} catch (err) {
			toastError(err, $_('common.error_generic'));
		} finally {
			loading = false;
		}
	}

	onMount(load);

	async function add() {
		const d = newDomain.trim().toLowerCase();
		fieldError = '';
		if (!d || !re.test(d) || d.length > 253) {
			fieldError = $_('status_pages.domains.invalid_format');
			return;
		}
		saving = true;
		try {
			const row = await addStatusPageDomain(statusPageId, d);
			domains = [...domains, row].sort((a, b) => a.domain.localeCompare(b.domain));
			newDomain = '';
			toastSuccess($_('status_pages.domains.added'));
		} catch (err) {
			if (err instanceof ApiError && err.status === 409) {
				fieldError = $_('status_pages.domains.conflict');
			} else if (err instanceof ApiError && err.fields?.domain) {
				fieldError = $_('status_pages.domains.invalid_format');
			} else {
				toastError(err, $_('common.error_generic'));
			}
		} finally {
			saving = false;
		}
	}

	async function remove(d: StatusPageDomain) {
		if (!confirm($_('status_pages.domains.confirm_remove'))) return;
		try {
			await deleteStatusPageDomain(statusPageId, d.id);
			domains = domains.filter((x) => x.id !== d.id);
			toastSuccess($_('status_pages.domains.removed'));
		} catch (err) {
			toastError(err, $_('common.error_generic'));
		}
	}
</script>

<section class="space-y-3">
	<div>
		<h2 class="text-base font-medium">{$_('status_pages.domains.title')}</h2>
		<p class="text-xs text-muted-foreground">{$_('status_pages.domains.help')}</p>
	</div>

	{#if loading}
		<p class="text-sm text-muted-foreground">…</p>
	{:else if domains.length === 0}
		<p class="text-sm text-muted-foreground">{$_('status_pages.domains.empty')}</p>
	{:else}
		<ul class="divide-y rounded-md border">
			{#each domains as d (d.id)}
				<li class="flex items-center justify-between px-3 py-2">
					<span class="font-mono text-sm">{d.domain}</span>
					<Button variant="ghost" size="sm" onclick={() => remove(d)}>
						{$_('status_pages.domains.remove')}
					</Button>
				</li>
			{/each}
		</ul>
	{/if}

	<div class="flex items-end gap-2">
		<div class="flex-1 space-y-1.5">
			<Label for="new-domain">{$_('status_pages.domains.add')}</Label>
			<Input
				id="new-domain"
				bind:value={newDomain}
				placeholder={$_('status_pages.domains.placeholder')}
				maxlength={253}
				onkeydown={(e: KeyboardEvent) => {
					if (e.key === 'Enter') {
						e.preventDefault();
						add();
					}
				}}
			/>
			{#if fieldError}
				<p class="text-xs text-destructive">{fieldError}</p>
			{/if}
		</div>
		<Button onclick={add} disabled={saving || !newDomain.trim()}>
			{$_('status_pages.domains.add')}
		</Button>
	</div>
</section>
