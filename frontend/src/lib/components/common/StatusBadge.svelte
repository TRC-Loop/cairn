<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<script lang="ts">
	import { _ } from 'svelte-i18n';
	import { Badge } from '$lib/components/ui/badge';

	type Props = {
		status: string;
		showDot?: boolean;
	};

	let { status, showDot = true }: Props = $props();

	const colorVar = $derived.by(() => {
		switch (status) {
			case 'up':
				return 'var(--status-up)';
			case 'degraded':
				return 'var(--status-degraded)';
			case 'down':
				return 'var(--status-down)';
			case 'paused':
				return 'var(--status-maintenance)';
			default:
				return 'var(--status-unknown)';
		}
	});
</script>

<Badge variant="outline" style="color: {colorVar}; border-color: {colorVar};">
	{#if showDot}
		<span
			class="mr-1.5 inline-block h-1.5 w-1.5 rounded-full"
			style="background-color: {colorVar};"
			aria-hidden="true"
		></span>
	{/if}
	{$_(`monitors.status.${status}`)}
</Badge>
