<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<script lang="ts">
	import type { Snippet } from 'svelte';

	type Props = {
		title: string;
		description?: string;
		actions?: Snippet;
	};

	let { title, description, actions }: Props = $props();
</script>

<header class="mb-6 flex items-start justify-between gap-4 border-b border-border pb-4">
	<div>
		<h1 class="text-xl font-medium text-foreground">{title}</h1>
		{#if description}
			<p class="mt-1 text-sm text-muted-foreground">{description}</p>
		{/if}
	</div>
	{#if actions}
		<div class="flex items-center gap-2">
			{@render actions()}
		</div>
	{/if}
</header>
