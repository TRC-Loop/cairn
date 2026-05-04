<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<script lang="ts">
	import { onMount } from 'svelte';
	import { _ } from 'svelte-i18n';
	import { versionInfo } from '$lib/stores/version';

	onMount(() => { versionInfo.load(); });
</script>

<div class="space-y-8">
	<section>
		<h2 class="mb-2 text-base font-medium">{$_('settings.about.section')}</h2>
		<p class="mb-4 text-sm text-muted-foreground">{$_('settings.about.tagline')}</p>
		<dl class="grid grid-cols-[max-content_1fr] gap-x-6 gap-y-2 text-sm">
			<dt class="text-muted-foreground">{$_('settings.about.version')}</dt>
			<dd class="font-mono text-foreground">{$versionInfo.version}</dd>
			<dt class="text-muted-foreground">{$_('settings.about.license')}</dt>
			<dd>AGPL-3.0-or-later</dd>
			<dt class="text-muted-foreground">{$_('settings.about.source')}</dt>
			<dd>
				<a
					class="text-primary hover:underline"
					href="https://github.com/TRC-Loop/cairn"
					target="_blank"
					rel="noopener noreferrer">github.com/TRC-Loop/cairn</a
				>
			</dd>
		</dl>
	</section>

	<section>
		<h2 class="mb-2 text-base font-medium">{$_('settings.about.privacy_title')}</h2>
		<p class="text-sm text-muted-foreground">{$_('settings.about.privacy_body')}</p>
	</section>
</div>
