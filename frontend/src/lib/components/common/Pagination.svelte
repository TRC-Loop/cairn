<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<script lang="ts">
	import { _ } from 'svelte-i18n';
	import { Button } from '$lib/components/ui/button';

	type Props = {
		page: number;
		pageSize: number;
		total: number;
		onPageChange: (p: number) => void;
	};

	let { page, pageSize, total, onPageChange }: Props = $props();

	const pageCount = $derived(Math.max(1, Math.ceil(total / pageSize)));
	const start = $derived(total === 0 ? 0 : (page - 1) * pageSize + 1);
	const end = $derived(Math.min(page * pageSize, total));
</script>

{#if total > pageSize}
	<div class="mt-4 flex items-center justify-between text-xs text-muted-foreground">
		<span>{start}-{end} / {total}</span>
		<div class="flex items-center gap-2">
			<Button
				variant="secondary"
				size="sm"
				disabled={page <= 1}
				onclick={() => onPageChange(page - 1)}
			>
				{$_('common.previous')}
			</Button>
			<span class="px-1 text-foreground">{page} / {pageCount}</span>
			<Button
				variant="secondary"
				size="sm"
				disabled={page >= pageCount}
				onclick={() => onPageChange(page + 1)}
			>
				{$_('common.next')}
			</Button>
		</div>
	</div>
{/if}
