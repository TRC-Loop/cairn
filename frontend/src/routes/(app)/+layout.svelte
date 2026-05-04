<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<script lang="ts">
	import { onMount } from 'svelte';
	import { _ } from 'svelte-i18n';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { auth } from '$lib/stores/auth';
	import {
		IconChartDots3,
		IconActivity,
		IconAlertTriangle,
		IconPageBreak,
		IconCategory,
		IconCalendar,
		IconBell,
		IconSettings,
		IconLogout,
		IconLayoutSidebarLeftCollapse,
		IconLayoutSidebarLeftExpand,
		IconExternalLink,
		IconBook,
		IconBrandGithub
	} from '@tabler/icons-svelte';
	import { Separator } from '$lib/components/ui/separator';
	import {
		Tooltip,
		TooltipTrigger,
		TooltipContent,
		TooltipProvider
	} from '$lib/components/ui/tooltip';

	let { data, children } = $props();

	let collapsed = $state(false);
	let mounted = $state(false);

	onMount(() => {
		try {
			collapsed = localStorage.getItem('cairn_sidebar_collapsed') === '1';
		} catch {
			collapsed = false;
		}
		mounted = true;

		const updateFavicon = async () => {
			try {
				const res = await fetch('/api/status/summary');
				if (!res.ok) return;
				const data = await res.json();
				const status = data.in_maintenance
					? 'maintenance'
					: data.overall_status === 'partial_outage' || data.overall_status === 'major_outage'
						? 'outage'
						: data.overall_status;
				let link = document.querySelector<HTMLLinkElement>('link[rel="icon"]');
				if (!link) {
					link = document.createElement('link');
					link.rel = 'icon';
					document.head.appendChild(link);
				}
				link.href = `/favicons/${status}.ico?v=${Date.now()}`;
			} catch {
				/* ignore */
			}
		};
		updateFavicon();
		const interval = setInterval(updateFavicon, 30_000);
		return () => clearInterval(interval);
	});

	function toggleCollapsed() {
		collapsed = !collapsed;
		try {
			localStorage.setItem('cairn_sidebar_collapsed', collapsed ? '1' : '0');
		} catch {
			/* ignore */
		}
	}

	const navItems = [
		{ href: '/dashboard', icon: IconChartDots3, labelKey: 'nav.dashboard', match: '/dashboard' },
		{ href: '/monitors', icon: IconActivity, labelKey: 'nav.monitors', match: '/monitors' },
		{
			href: '/incidents',
			icon: IconAlertTriangle,
			labelKey: 'nav.incidents',
			match: '/incidents'
		},
		{
			href: '/status-pages',
			icon: IconPageBreak,
			labelKey: 'nav.status_pages',
			match: '/status-pages'
		},
		{
			href: '/components',
			icon: IconCategory,
			labelKey: 'nav.components',
			match: '/components'
		},
		{
			href: '/maintenance',
			icon: IconCalendar,
			labelKey: 'nav.maintenance',
			match: '/maintenance'
		},
		{
			href: '/notifications',
			icon: IconBell,
			labelKey: 'nav.notifications',
			match: '/notifications'
		}
	];

	function isActive(match: string, pathname: string) {
		if (match === '/dashboard') return pathname === '/dashboard';
		return pathname.startsWith(match);
	}

	async function signOut() {
		await auth.logout();
		await goto('/login');
	}
</script>

<TooltipProvider>
	<div class="flex min-h-screen bg-background text-foreground">
		<aside
			class="flex shrink-0 flex-col overflow-hidden border-r border-border bg-sidebar motion-safe:transition-[width] motion-safe:duration-200"
			style="width: {collapsed ? '56px' : '200px'}; border-right-width: 0.5px;"
		>
			<div class="flex items-center justify-between px-3 py-4">
				{#if !collapsed}
					<span class="text-sm font-medium tracking-tight">cairn</span>
				{/if}
				<Tooltip>
					<TooltipTrigger>
						{#snippet child({ props })}
							<button
								{...props}
								type="button"
								class="rounded-md p-1 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
								aria-label={collapsed
									? $_('common.expand_sidebar')
									: $_('common.collapse_sidebar')}
								onclick={toggleCollapsed}
							>
								{#if collapsed}
									<IconLayoutSidebarLeftExpand size={16} />
								{:else}
									<IconLayoutSidebarLeftCollapse size={16} />
								{/if}
							</button>
						{/snippet}
					</TooltipTrigger>
					{#if collapsed}
						<TooltipContent side="right">
							{$_('common.expand_sidebar')}
						</TooltipContent>
					{/if}
				</Tooltip>
			</div>

			<nav class="flex flex-1 flex-col gap-0.5 px-2 py-2">
				{#each navItems as item (item.href)}
					{@const active = isActive(item.match, page.url.pathname)}
					<Tooltip>
						<TooltipTrigger>
							{#snippet child({ props })}
								<a
									href={item.href}
									class="relative flex items-center gap-2 rounded-md px-3 py-1.5 text-sm transition-colors hover:bg-accent {active
										? 'text-foreground'
										: 'text-muted-foreground'}"
									{...props}
								>
									{#if active}
										<span
											class="absolute top-1.5 bottom-1.5 left-0 w-0.5 rounded-full bg-primary"
											aria-hidden="true"
										></span>
									{/if}
									<item.icon size={16} class="shrink-0" />
									{#if !collapsed}
										<span class="truncate">{$_(item.labelKey)}</span>
									{/if}
								</a>
							{/snippet}
						</TooltipTrigger>
						{#if collapsed}
							<TooltipContent side="right">
								{$_(item.labelKey)}
							</TooltipContent>
						{/if}
					</Tooltip>
				{/each}
			</nav>

			<Separator class="my-1" />

			<div class="px-2 py-2">
				<Tooltip>
					<TooltipTrigger>
						{#snippet child({ props })}
							<a
								href="/settings"
								class="relative flex items-center gap-2 rounded-md px-3 py-1.5 text-sm transition-colors hover:bg-accent {isActive(
									'/settings',
									page.url.pathname
								)
									? 'text-foreground'
									: 'text-muted-foreground'}"
								{...props}
							>
								{#if isActive('/settings', page.url.pathname)}
									<span
										class="absolute top-1.5 bottom-1.5 left-0 w-0.5 rounded-full bg-primary"
										aria-hidden="true"
									></span>
								{/if}
								<IconSettings size={16} class="shrink-0" />
								{#if !collapsed}
									<span class="truncate">{$_('nav.settings')}</span>
								{/if}
							</a>
						{/snippet}
					</TooltipTrigger>
					{#if collapsed}
						<TooltipContent side="right">{$_('nav.settings')}</TooltipContent>
					{/if}
				</Tooltip>
			</div>

			<div class="p-2">
				<div
					class="flex items-center gap-2 rounded-md px-2 py-2 text-sm"
					class:justify-between={!collapsed}
					class:justify-center={collapsed}
				>
					{#if !collapsed}
						<div class="flex min-w-0 flex-col">
							<span class="truncate text-foreground">{data.user.display_name}</span>
							<span class="truncate text-xs text-muted-foreground">{data.user.role}</span>
						</div>
					{/if}
					<Tooltip>
						<TooltipTrigger>
							{#snippet child({ props })}
								<button
									{...props}
									type="button"
									class="rounded-md p-1.5 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
									aria-label={$_('common.logout')}
									onclick={signOut}
								>
									<IconLogout size={16} />
								</button>
							{/snippet}
						</TooltipTrigger>
						<TooltipContent side={collapsed ? 'right' : 'top'}>
							{$_('common.logout')}
						</TooltipContent>
					</Tooltip>
				</div>
			</div>

			<Separator class="my-1" />

			<div class="flex flex-col gap-1 px-2 pb-3 pt-1">
				{#if !collapsed}
					<span class="px-3 text-[11px] text-muted-foreground">Cairn v0.1.0-dev</span>
				{/if}
				<Tooltip>
					<TooltipTrigger>
						{#snippet child({ props })}
							<a
								{...props}
								href="https://github.com/TRC-Loop/cairn"
								target="_blank"
								rel="noopener noreferrer"
								class="flex items-center gap-2 rounded-md px-3 py-1 text-xs text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
							>
								<IconBrandGithub size={14} class="shrink-0" />
								{#if !collapsed}
									<span class="flex-1 truncate">{$_('common.source')}</span>
									<IconExternalLink size={11} class="shrink-0 opacity-60" />
								{/if}
							</a>
						{/snippet}
					</TooltipTrigger>
					{#if collapsed}
						<TooltipContent side="right">{$_('common.source')}</TooltipContent>
					{/if}
				</Tooltip>
				<Tooltip>
					<TooltipTrigger>
						{#snippet child({ props })}
							<a
								{...props}
								href="https://cairn.arne.sh"
								target="_blank"
								rel="noopener noreferrer"
								class="flex items-center gap-2 rounded-md px-3 py-1 text-xs text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
							>
								<IconBook size={14} class="shrink-0" />
								{#if !collapsed}
									<span class="flex-1 truncate">{$_('common.docs')}</span>
									<IconExternalLink size={11} class="shrink-0 opacity-60" />
								{/if}
							</a>
						{/snippet}
					</TooltipTrigger>
					{#if collapsed}
						<TooltipContent side="right">{$_('common.docs')}</TooltipContent>
					{/if}
				</Tooltip>
			</div>
		</aside>

		<main class="flex-1 overflow-auto">
			{@render children()}
		</main>
	</div>
</TooltipProvider>
