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
	import { createComponent, updateComponent, type Component } from '$lib/api/components';
	import { ApiError } from '$lib/api/client';
	import { toastError, toastSuccess } from '$lib/toast';
	import { fieldErrorText } from '$lib/validate';

	type Props = {
		open: boolean;
		mode: 'create' | 'edit';
		existing?: Component | null;
		onOpenChange: (v: boolean) => void;
		onSaved: () => void;
	};

	let { open, mode, existing = null, onOpenChange, onSaved }: Props = $props();

	let name = $state('');
	let description = $state('');
	let saving = $state(false);
	let fieldErrors = $state<Record<string, string>>({});

	$effect(() => {
		if (open) {
			if (mode === 'edit' && existing) {
				name = existing.name;
				description = existing.description ?? '';
			} else {
				name = '';
				description = '';
			}
			fieldErrors = {};
		}
	});

	async function submit() {
		fieldErrors = {};
		if (!name.trim()) {
			fieldErrors = { name: $_('common.required') };
			return;
		}
		saving = true;
		try {
			const payload = {
				name: name.trim(),
				description: description.trim()
			};
			if (mode === 'edit' && existing) {
				await updateComponent(existing.id, payload);
				toastSuccess($_('components.edit.success'));
			} else {
				await createComponent(payload);
				toastSuccess($_('components.create.success'));
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
	<DialogContent class="sm:max-w-[480px]">
		<DialogHeader>
			<DialogTitle>
				{mode === 'edit' ? $_('components.edit.title') : $_('components.create.title')}
			</DialogTitle>
		</DialogHeader>

		<div class="space-y-4 py-2">
			<div class="space-y-1.5">
				<Label for="comp-name">{$_('components.fields.name')}</Label>
				<Input id="comp-name" bind:value={name} maxlength={100} />
				{#if fieldErrors.name}
					<p class="text-xs text-destructive">{fieldErrorText(fieldErrors.name)}</p>
				{/if}
			</div>

			<div class="space-y-1.5">
				<Label for="comp-desc">{$_('components.fields.description')}</Label>
				<Input id="comp-desc" bind:value={description} maxlength={500} />
				{#if fieldErrors.description}
					<p class="text-xs text-destructive">{fieldErrorText(fieldErrors.description)}</p>
				{/if}
			</div>
		</div>

		<DialogFooter>
			<Button variant="secondary" onclick={() => onOpenChange(false)} disabled={saving}>
				{$_('common.cancel')}
			</Button>
			<Button onclick={submit} disabled={saving}>
				{saving
					? $_('common.saving')
					: mode === 'edit'
						? $_('components.edit.submit')
						: $_('components.create.submit')}
			</Button>
		</DialogFooter>
	</DialogContent>
</Dialog>
