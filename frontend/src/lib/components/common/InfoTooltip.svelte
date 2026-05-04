<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<script lang="ts">
	import * as Tooltip from '$lib/components/ui/tooltip';
	import { IconInfoCircle } from '@tabler/icons-svelte';

	type Props = { text: string };
	let { text }: Props = $props();
</script>

<Tooltip.Provider>
	<Tooltip.Root>
		<Tooltip.Trigger
			type="button"
			class="text-muted-foreground hover:text-foreground inline-flex items-center"
			aria-label="More info"
		>
			<IconInfoCircle size={14} />
		</Tooltip.Trigger>
		<Tooltip.Content class="max-w-xs">
			<p>{text}</p>
		</Tooltip.Content>
	</Tooltip.Root>
</Tooltip.Provider>
