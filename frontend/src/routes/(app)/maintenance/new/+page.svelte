<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<script lang="ts">
	import { _ } from 'svelte-i18n';
	import { goto } from '$app/navigation';
	import PageHeader from '$lib/components/common/PageHeader.svelte';
	import MaintenanceForm from '../MaintenanceForm.svelte';
	import { createMaintenance } from '$lib/api/maintenance';
	import { toastError, toastSuccess } from '$lib/toast';

	function defaultStart(): string {
		const d = new Date(Date.now() + 60 * 60 * 1000);
		return toLocalInput(d);
	}
	function defaultEnd(): string {
		const d = new Date(Date.now() + 2 * 60 * 60 * 1000);
		return toLocalInput(d);
	}
	function toLocalInput(d: Date): string {
		const pad = (n: number) => n.toString().padStart(2, '0');
		return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
	}
	function localToUTC(local: string): string {
		return new Date(local).toISOString();
	}

	let title = $state('');
	let description = $state('');
	let startsAtLocal = $state(defaultStart());
	let endsAtLocal = $state(defaultEnd());
	let selected = $state<Set<number>>(new Set());
	let saving = $state(false);

	async function submit() {
		if (!title.trim()) {
			toastError(null, $_('common.required'));
			return;
		}
		if (selected.size === 0) {
			toastError(null, $_('maintenance.fields.affected_required'));
			return;
		}
		saving = true;
		try {
			const win = await createMaintenance({
				title: title.trim(),
				description: description.trim(),
				starts_at: localToUTC(startsAtLocal),
				ends_at: localToUTC(endsAtLocal),
				affected_component_ids: [...selected]
			});
			toastSuccess($_('maintenance.create.success'));
			void goto(`/maintenance/${win.id}`);
		} catch (err) {
			toastError(err, $_('common.error_generic'));
		} finally {
			saving = false;
		}
	}
</script>

<div class="p-6">
	<a href="/maintenance" class="mb-4 inline-block text-sm text-muted-foreground hover:text-foreground">
		← {$_('maintenance.detail.back')}
	</a>
	<PageHeader title={$_('maintenance.create.title')} />

	<MaintenanceForm
		mode="create"
		bind:title
		bind:description
		bind:startsAtLocal
		bind:endsAtLocal
		bind:selected
		{saving}
		onsubmit={submit}
		oncancel={() => goto('/maintenance')}
	/>
</div>
