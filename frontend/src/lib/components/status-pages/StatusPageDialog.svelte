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
	import { Label } from '$lib/components/ui/label';
	import {
		createStatusPage,
		updateStatusPage,
		type StatusPage
	} from '$lib/api/statusPages';
	import { ApiError } from '$lib/api/client';
	import { toastError, toastSuccess } from '$lib/toast';
	import { fieldErrorText } from '$lib/validate';

	type Props = {
		open: boolean;
		mode: 'create' | 'edit';
		existing?: StatusPage | null;
		onOpenChange: (v: boolean) => void;
		onSaved: () => void;
	};

	let { open, mode, existing = null, onOpenChange, onSaved }: Props = $props();

	let slug = $state('');
	let title = $state('');
	let description = $state('');
	let logoUrl = $state('');
	let accentColor = $state('');
	let isDefault = $state(false);
	let saving = $state(false);
	let fieldErrors = $state<Record<string, string>>({});

	$effect(() => {
		if (open) {
			if (mode === 'edit' && existing) {
				slug = existing.slug;
				title = existing.title;
				description = existing.description;
				logoUrl = existing.logo_url;
				accentColor = existing.accent_color;
				isDefault = existing.is_default;
			} else {
				slug = '';
				title = '';
				description = '';
				logoUrl = '';
				accentColor = '';
				isDefault = false;
			}
			fieldErrors = {};
		}
	});

	async function submit() {
		fieldErrors = {};
		const errs: Record<string, string> = {};
		if (!title.trim()) errs.title = $_('common.required');
		if (mode === 'create') {
			if (!slug.trim()) errs.slug = $_('common.required');
			else if (!/^[a-z0-9](?:[a-z0-9-]{0,62}[a-z0-9])?$/.test(slug.trim())) {
				errs.slug = $_('status_pages.fields.slug_help');
			}
		}
		if (Object.keys(errs).length > 0) {
			fieldErrors = errs;
			return;
		}
		saving = true;
		try {
			if (mode === 'edit' && existing) {
				await updateStatusPage(existing.id, {
					title: title.trim(),
					description: description.trim(),
					logo_url: logoUrl.trim(),
					accent_color: accentColor.trim()
				});
				toastSuccess($_('status_pages.edit.success'));
			} else {
				await createStatusPage({
					slug: slug.trim(),
					title: title.trim(),
					description: description.trim(),
					logo_url: logoUrl.trim(),
					accent_color: accentColor.trim(),
					is_default: isDefault
				});
				toastSuccess($_('status_pages.create.success'));
			}
			onSaved();
			onOpenChange(false);
		} catch (err) {
			if (err instanceof ApiError && err.fields) {
				fieldErrors = err.fields;
			} else {
				toastError(err, $_('common.error_generic'));
			}
		} finally {
			saving = false;
		}
	}
</script>

<Dialog {open} {onOpenChange}>
	<DialogContent class="sm:max-w-[560px]">
		<DialogHeader>
			<DialogTitle>
				{mode === 'edit' ? $_('status_pages.edit.title') : $_('status_pages.create.title')}
			</DialogTitle>
		</DialogHeader>

		<div class="max-h-[60vh] space-y-4 overflow-y-auto py-2 pr-1">
			<div class="space-y-1.5">
				<Label for="sp-title">{$_('status_pages.fields.title')}</Label>
				<Input id="sp-title" bind:value={title} maxlength={200} />
				{#if fieldErrors.title}
					<p class="text-xs text-destructive">{fieldErrorText(fieldErrors.title)}</p>
				{/if}
			</div>

			{#if mode === 'create'}
				<div class="space-y-1.5">
					<Label for="sp-slug">{$_('status_pages.fields.slug')}</Label>
					<Input id="sp-slug" bind:value={slug} maxlength={64} />
					<p class="text-xs text-muted-foreground">{$_('status_pages.fields.slug_help')}</p>
					{#if fieldErrors.slug}
						<p class="text-xs text-destructive">{fieldErrorText(fieldErrors.slug)}</p>
					{/if}
				</div>
			{/if}

			<div class="space-y-1.5">
				<Label for="sp-desc">{$_('status_pages.fields.description')}</Label>
				<Input id="sp-desc" bind:value={description} maxlength={500} />
			</div>

			<div class="space-y-1.5">
				<Label for="sp-logo">{$_('status_pages.fields.logo_url')}</Label>
				<Input id="sp-logo" bind:value={logoUrl} type="url" />
			</div>

			<div class="space-y-1.5">
				<Label for="sp-accent">{$_('status_pages.fields.accent_color')}</Label>
				<Input id="sp-accent" bind:value={accentColor} placeholder="#7c8ea2" />
			</div>

{#if mode === 'create'}
				<label class="flex items-center gap-2 text-sm">
					<input type="checkbox" bind:checked={isDefault} class="rounded border-input" />
					<span>{$_('status_pages.fields.is_default')}</span>
				</label>
				{#if isDefault}
					<p class="text-xs text-muted-foreground">
						{$_('status_pages.fields.is_default_replace')}
					</p>
				{/if}
			{/if}
		</div>

		<DialogFooter>
			<Button variant="secondary" onclick={() => onOpenChange(false)} disabled={saving}>
				{$_('common.cancel')}
			</Button>
			<Button onclick={submit} disabled={saving}>
				{saving
					? $_('common.saving')
					: mode === 'edit'
						? $_('status_pages.edit.submit')
						: $_('status_pages.create.submit')}
			</Button>
		</DialogFooter>
	</DialogContent>
</Dialog>
