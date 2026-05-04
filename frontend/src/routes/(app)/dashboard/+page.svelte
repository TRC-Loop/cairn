<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<script lang="ts">
	import { _ } from 'svelte-i18n';
	import { IconActivity, IconPageBreak, IconCategory } from '@tabler/icons-svelte';
	import PageHeader from '$lib/components/common/PageHeader.svelte';

	const cards = [
		{
			href: '/monitors/new',
			icon: IconActivity,
			titleKey: 'dashboard.add_monitor_title',
			descKey: 'dashboard.add_monitor_desc'
		},
		{
			href: '/components/new',
			icon: IconCategory,
			titleKey: 'dashboard.create_component_title',
			descKey: 'dashboard.create_component_desc'
		},
		{
			href: '/status-pages/new',
			icon: IconPageBreak,
			titleKey: 'dashboard.create_status_page_title',
			descKey: 'dashboard.create_status_page_desc'
		}
	];
</script>

<div class="p-6">
	<PageHeader title={$_('dashboard.title')} />

	<section>
		<h2 class="mb-3 text-sm font-medium text-foreground">
			{$_('dashboard.first_steps')}
		</h2>
		<div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
			{#each cards as card (card.href)}
				<a
					href={card.href}
					class="group flex flex-col gap-2 rounded-md border border-border p-4 transition-colors hover:bg-accent"
				>
					<div class="flex items-center gap-2 text-foreground">
						<card.icon size={16} class="text-muted-foreground" />
						<span class="text-sm font-medium">{$_(card.titleKey)}</span>
					</div>
					<p class="text-sm text-muted-foreground">{$_(card.descKey)}</p>
				</a>
			{/each}
		</div>
	</section>
</div>
