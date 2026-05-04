<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<script lang="ts">
	import { _ } from 'svelte-i18n';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Select from '$lib/components/ui/select';
	import {
		Dialog,
		DialogContent,
		DialogHeader,
		DialogTitle,
		DialogFooter
	} from '$lib/components/ui/dialog';
	import {
		IconArrowUp,
		IconArrowDown,
		IconLink,
		IconLetterCase,
		IconMinus,
		IconTrash,
		IconPencil,
		IconPlus
	} from '@tabler/icons-svelte';
	import {
		getStatusPageFooter,
		replaceFooterElements,
		setStatusPageFooterMode,
		updateStatusPage,
		type FooterMode,
		type FooterElement,
		type FooterElementInput,
		type FooterElementType
	} from '$lib/api/statusPages';
	import { ApiError } from '$lib/api/client';
	import { toastError, toastSuccess } from '$lib/toast';

	type Props = { statusPageId: number };
	let { statusPageId }: Props = $props();

	let loading = $state(true);
	let mode = $state<FooterMode>('structured');
	let elements = $state<FooterElement[]>([]);
	let customHtml = $state('');
	let savingHtml = $state(false);

	let dialogOpen = $state(false);
	let dialogType = $state<FooterElementType>('link');
	let editingIndex = $state<number | null>(null);
	let formLabel = $state('');
	let formUrl = $state('');
	let formNewTab = $state(true);
	let formError = $state<string | null>(null);

	async function load() {
		loading = true;
		try {
			const data = await getStatusPageFooter(statusPageId);
			mode = data.footer_mode;
			elements = data.elements;
			customHtml = data.custom_footer_html ?? '';
		} catch (err) {
			toastError(err, $_('common.error_generic'));
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		void statusPageId;
		load();
	});

	async function changeMode(value: string) {
		const next = value as FooterMode;
		if (next === mode) return;
		const prev = mode;
		mode = next;
		try {
			await setStatusPageFooterMode(statusPageId, next);
			toastSuccess($_('status_pages.footer.saved'));
		} catch (err) {
			mode = prev;
			toastError(err, $_('common.error_generic'));
		}
	}

	function elementsToInputs(list: FooterElement[]): FooterElementInput[] {
		return list.map((e) => ({
			element_type: e.element_type,
			label: e.label ?? '',
			url: e.url ?? '',
			open_in_new_tab: e.open_in_new_tab
		}));
	}

	async function persist(next: FooterElementInput[]) {
		try {
			const saved = await replaceFooterElements(statusPageId, next);
			elements = saved;
			toastSuccess($_('status_pages.footer.saved'));
		} catch (err) {
			if (err instanceof ApiError && err.fields) {
				const first = Object.values(err.fields)[0];
				toastError(err, first ?? $_('common.error_generic'));
			} else {
				toastError(err, $_('common.error_generic'));
			}
			await load();
		}
	}

	function openAddLink() {
		editingIndex = null;
		dialogType = 'link';
		formLabel = '';
		formUrl = '';
		formNewTab = true;
		formError = null;
		dialogOpen = true;
	}
	function openAddText() {
		editingIndex = null;
		dialogType = 'text';
		formLabel = '';
		formUrl = '';
		formError = null;
		dialogOpen = true;
	}
	async function addSeparator() {
		const next = elementsToInputs(elements);
		next.push({ element_type: 'separator' });
		await persist(next);
	}

	function openEdit(idx: number) {
		const el = elements[idx];
		if (el.element_type === 'separator') return;
		editingIndex = idx;
		dialogType = el.element_type;
		formLabel = el.label ?? '';
		formUrl = el.url ?? '';
		formNewTab = el.open_in_new_tab;
		formError = null;
		dialogOpen = true;
	}

	async function submitDialog() {
		formError = null;
		const trimmedLabel = formLabel.trim();
		const trimmedUrl = formUrl.trim();
		if (dialogType === 'link') {
			if (!trimmedLabel) {
				formError = $_('status_pages.footer.error.label_required');
				return;
			}
			if (trimmedLabel.length > 100) {
				formError = $_('status_pages.footer.error.label_too_long');
				return;
			}
			if (!trimmedUrl) {
				formError = $_('status_pages.footer.error.url_required');
				return;
			}
			if (!/^(https?:|mailto:)/i.test(trimmedUrl)) {
				formError = $_('status_pages.footer.error.url_invalid');
				return;
			}
		} else if (dialogType === 'text') {
			if (!trimmedLabel) {
				formError = $_('status_pages.footer.error.label_required');
				return;
			}
			if (trimmedLabel.length > 200) {
				formError = $_('status_pages.footer.error.label_too_long');
				return;
			}
		}
		const next = elementsToInputs(elements);
		const entry: FooterElementInput = {
			element_type: dialogType,
			label: trimmedLabel,
			url: dialogType === 'link' ? trimmedUrl : '',
			open_in_new_tab: dialogType === 'link' ? formNewTab : false
		};
		if (editingIndex !== null) {
			next[editingIndex] = entry;
		} else {
			next.push(entry);
		}
		dialogOpen = false;
		await persist(next);
	}

	async function moveUp(idx: number) {
		if (idx <= 0) return;
		const next = elementsToInputs(elements);
		[next[idx - 1], next[idx]] = [next[idx], next[idx - 1]];
		await persist(next);
	}
	async function moveDown(idx: number) {
		if (idx >= elements.length - 1) return;
		const next = elementsToInputs(elements);
		[next[idx], next[idx + 1]] = [next[idx + 1], next[idx]];
		await persist(next);
	}
	async function removeAt(idx: number) {
		const next = elementsToInputs(elements).filter((_, i) => i !== idx);
		await persist(next);
	}

	async function saveCustomHtml() {
		savingHtml = true;
		try {
			await updateStatusPage(statusPageId, { custom_footer_html: customHtml });
			toastSuccess($_('status_pages.footer.saved'));
		} catch (err) {
			toastError(err, $_('common.error_generic'));
		} finally {
			savingHtml = false;
		}
	}

	const modeLabel = $derived(
		mode === 'structured'
			? $_('status_pages.footer.mode_structured')
			: mode === 'html'
				? $_('status_pages.footer.mode_html')
				: $_('status_pages.footer.mode_both')
	);
</script>

<div class="rounded-md border border-border p-4">
	<div class="mb-3 flex items-center justify-between">
		<h2 class="text-sm font-medium text-foreground">{$_('status_pages.footer.title')}</h2>
		<div class="w-44">
			<Select.Root type="single" value={mode} onValueChange={changeMode}>
				<Select.Trigger class="w-full">{modeLabel}</Select.Trigger>
				<Select.Content>
					<Select.Item value="structured">
						{$_('status_pages.footer.mode_structured')}
					</Select.Item>
					<Select.Item value="html">{$_('status_pages.footer.mode_html')}</Select.Item>
					<Select.Item value="both">{$_('status_pages.footer.mode_both')}</Select.Item>
				</Select.Content>
			</Select.Root>
		</div>
	</div>

	{#if loading}
		<p class="text-sm text-muted-foreground">{$_('common.loading')}</p>
	{:else}
		{#if mode === 'structured' || mode === 'both'}
			<div class="mb-4">
				<p class="mb-2 text-xs text-muted-foreground">
					{$_('status_pages.footer.elements_help')}
				</p>
				{#if elements.length === 0}
					<p class="text-sm text-muted-foreground">
						{$_('status_pages.footer.empty')}
					</p>
				{:else}
					<ul class="divide-y divide-border rounded-md border border-border">
						{#each elements as el, idx (el.id)}
							<li class="flex items-center justify-between gap-2 px-3 py-2 text-sm">
								<div class="flex min-w-0 items-center gap-2">
									{#if el.element_type === 'link'}
										<IconLink size={14} class="shrink-0 text-muted-foreground" />
										<span class="truncate text-foreground">{el.label}</span>
										<span class="truncate text-xs text-muted-foreground">{el.url}</span>
									{:else if el.element_type === 'text'}
										<IconLetterCase size={14} class="shrink-0 text-muted-foreground" />
										<span class="truncate text-foreground">{el.label}</span>
									{:else}
										<IconMinus size={14} class="shrink-0 text-muted-foreground" />
										<span class="text-muted-foreground">—</span>
									{/if}
								</div>
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
										disabled={idx === elements.length - 1}
										onclick={() => moveDown(idx)}
									>
										<IconArrowDown size={14} />
									</button>
									{#if el.element_type !== 'separator'}
										<button
											type="button"
											aria-label={$_('common.edit')}
											class="inline-flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground hover:bg-accent hover:text-foreground"
											onclick={() => openEdit(idx)}
										>
											<IconPencil size={14} />
										</button>
									{/if}
									<button
										type="button"
										aria-label={$_('common.delete')}
										class="inline-flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground hover:bg-accent hover:text-destructive"
										onclick={() => removeAt(idx)}
									>
										<IconTrash size={14} />
									</button>
								</div>
							</li>
						{/each}
					</ul>
				{/if}
				<div class="mt-2 flex flex-wrap gap-2">
					<Button size="sm" variant="secondary" onclick={openAddLink}>
						<IconPlus size={14} class="mr-1" />
						{$_('status_pages.footer.add_link')}
					</Button>
					<Button size="sm" variant="secondary" onclick={openAddText}>
						<IconPlus size={14} class="mr-1" />
						{$_('status_pages.footer.add_text')}
					</Button>
					<Button size="sm" variant="secondary" onclick={addSeparator}>
						<IconPlus size={14} class="mr-1" />
						{$_('status_pages.footer.add_separator')}
					</Button>
				</div>
			</div>
		{/if}

		{#if mode === 'html' || mode === 'both'}
			<div class="space-y-1.5">
				<Label for="sp-footer-html">{$_('status_pages.footer.html_label')}</Label>
				<p class="text-xs text-muted-foreground">{$_('status_pages.footer.html_help')}</p>
				<textarea
					id="sp-footer-html"
					bind:value={customHtml}
					rows="4"
					class="w-full rounded-md border border-input bg-background px-3 py-2 font-mono text-sm focus:outline-none focus:ring-2 focus:ring-ring"
				></textarea>
				<div>
					<Button size="sm" onclick={saveCustomHtml} disabled={savingHtml}>
						{savingHtml ? $_('common.saving') : $_('common.save')}
					</Button>
				</div>
			</div>
		{/if}
	{/if}
</div>

<Dialog open={dialogOpen} onOpenChange={(v) => (dialogOpen = v)}>
	<DialogContent class="sm:max-w-[460px]">
		<DialogHeader>
			<DialogTitle>
				{dialogType === 'link'
					? $_('status_pages.footer.dialog_link')
					: $_('status_pages.footer.dialog_text')}
			</DialogTitle>
		</DialogHeader>
		<div class="space-y-3 py-2">
			<div class="space-y-1.5">
				<Label for="fe-label">{$_('status_pages.footer.element.label')}</Label>
				<Input
					id="fe-label"
					bind:value={formLabel}
					maxlength={dialogType === 'link' ? 100 : 200}
				/>
			</div>
			{#if dialogType === 'link'}
				<div class="space-y-1.5">
					<Label for="fe-url">{$_('status_pages.footer.element.url')}</Label>
					<Input id="fe-url" type="url" bind:value={formUrl} maxlength={500} />
				</div>
				<label class="flex items-center gap-2 text-sm">
					<input type="checkbox" bind:checked={formNewTab} class="rounded border-input" />
					<span>{$_('status_pages.footer.element.open_in_new_tab')}</span>
				</label>
			{/if}
			{#if formError}
				<p class="text-xs text-destructive">{formError}</p>
			{/if}
		</div>
		<DialogFooter>
			<Button variant="secondary" onclick={() => (dialogOpen = false)}>
				{$_('common.cancel')}
			</Button>
			<Button onclick={submitDialog}>{$_('common.save')}</Button>
		</DialogFooter>
	</DialogContent>
</Dialog>
