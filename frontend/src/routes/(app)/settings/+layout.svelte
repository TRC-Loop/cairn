<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<script lang="ts">
	import { _ } from 'svelte-i18n';
	import { page } from '$app/state';

	let { data, children } = $props();

	const isAdmin = $derived(data.user.role === 'admin');

	type Tab = { href: string; labelKey: string; adminOnly?: boolean };
	const tabs: Tab[] = [
		{ href: '/settings/profile', labelKey: 'settings.tabs.profile' },
		{ href: '/settings/users', labelKey: 'settings.tabs.users', adminOnly: true },
		{ href: '/settings/system', labelKey: 'settings.tabs.system', adminOnly: true },
		{ href: '/settings/retention', labelKey: 'settings.tabs.retention', adminOnly: true },
		{ href: '/settings/backup', labelKey: 'settings.tabs.backup', adminOnly: true },
		{ href: '/settings/about', labelKey: 'settings.tabs.about' }
	];

	const visibleTabs = $derived(tabs.filter((t) => !t.adminOnly || isAdmin));

	function isActive(href: string, pathname: string) {
		return pathname === href || pathname.startsWith(href + '/');
	}
</script>

<div class="mx-auto max-w-5xl px-6 py-8">
	<header class="mb-6 border-b border-border pb-4">
		<h1 class="text-xl font-medium text-foreground">{$_('settings.title')}</h1>
		<p class="mt-1 text-sm text-muted-foreground">{$_('settings.subtitle')}</p>
	</header>

	<nav class="mb-6 flex gap-1 border-b border-border" aria-label="Settings tabs">
		{#each visibleTabs as tab (tab.href)}
			{@const active = isActive(tab.href, page.url.pathname)}
			<a
				href={tab.href}
				class="-mb-px border-b-2 px-3 py-2 text-sm transition-colors {active
					? 'border-primary text-foreground'
					: 'border-transparent text-muted-foreground hover:text-foreground'}"
				aria-current={active ? 'page' : undefined}
			>
				{$_(tab.labelKey)}
			</a>
		{/each}
	</nav>

	{@render children()}
</div>
