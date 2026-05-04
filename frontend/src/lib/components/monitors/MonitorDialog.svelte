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
	import * as Select from '$lib/components/ui/select';
	import InfoTooltip from '$lib/components/common/InfoTooltip.svelte';
	import { createMonitor, updateMonitor } from '$lib/api/monitors';
	import type { Monitor, MonitorType, MonitorWriteInput, ReopenMode } from '$lib/api/monitors';
	import { listComponents, type Component } from '$lib/api/components';
	import { listChannels, type NotificationChannel } from '$lib/api/notifications';
	import { ApiError } from '$lib/api/client';
	import { toastError, toastSuccess } from '$lib/toast';
	import { fieldErrorText } from '$lib/validate';

	type Props = {
		open: boolean;
		mode: 'create' | 'edit';
		existing?: Monitor | null;
		preselectComponentId?: number | null;
		onOpenChange: (v: boolean) => void;
		onSaved: () => void;
	};

	let {
		open,
		mode,
		existing = null,
		preselectComponentId = null,
		onOpenChange,
		onSaved
	}: Props = $props();

	const TYPES: MonitorType[] = [
		'http',
		'tcp',
		'icmp',
		'dns',
		'tls',
		'push',
		'db_postgres',
		'db_mysql',
		'db_redis',
		'grpc'
	];

	let name = $state('');
	let type = $state<MonitorType>('http');
	let intervalSeconds = $state(60);
	let timeoutSeconds = $state(10);
	let failureThreshold = $state(3);
	let recoveryThreshold = $state(1);
	let componentId = $state<number | null>(null);
	let components = $state<Component[]>([]);
	let channels = $state<NotificationChannel[]>([]);
	let selectedChannelIds = $state<number[]>([]);

	// http
	let url = $state('');
	let method = $state('GET');
	let expectedStatusCodes = $state('200-299');
	// tcp/icmp/dns/tls/redis/grpc
	let host = $state('');
	let port = $state<number | ''>('');
	let count = $state(3);
	let recordType = $state('A');
	let resolver = $state('');
	let warnDays = $state(30);
	let criticalDays = $state(7);
	// db
	let dsn = $state('');
	let query = $state('SELECT 1');
	// redis
	let redisAddress = $state('');
	let redisPassword = $state('');
	let redisDb = $state(0);
	// grpc
	let grpcAddress = $state('');
	let grpcService = $state('');
	let grpcTls = $state(true);

	let advancedOpen = $state(false);
	let reopenOverrideEnabled = $state(false);
	let reopenWindowOverride = $state<number | ''>('');
	let reopenModeOverride = $state<string>('flapping_only');

	let submitting = $state(false);
	let fieldErrors = $state<Record<string, string>>({});

	const STATUS_CODES_RE = /^[0-9,\-\s]*$/;

	$effect(() => {
		if (open) {
			initFromExisting();
			fieldErrors = {};
			void loadComponents();
			void loadChannels();
		}
	});

	async function loadComponents() {
		try {
			components = await listComponents();
		} catch {
			components = [];
		}
	}

	async function loadChannels() {
		try {
			channels = (await listChannels()).filter((c) => c.enabled);
		} catch {
			channels = [];
		}
	}

	function toggleChannel(id: number) {
		selectedChannelIds = selectedChannelIds.includes(id)
			? selectedChannelIds.filter((x) => x !== id)
			: [...selectedChannelIds, id];
	}

	function initFromExisting() {
		if (mode === 'edit' && existing) {
			name = existing.name;
			type = existing.type;
			intervalSeconds = existing.interval_seconds;
			timeoutSeconds = existing.timeout_seconds;
			failureThreshold = existing.failure_threshold;
			recoveryThreshold = existing.recovery_threshold;
			componentId = existing.component_id;
			selectedChannelIds = [...(existing.notification_channel_ids ?? [])];
			const cfg = existing.config ?? {};
			const g = (k: string) => (cfg as Record<string, unknown>)[k];
			url = (g('url') as string) ?? '';
			method = (g('method') as string) ?? 'GET';
			expectedStatusCodes = (g('expected_status_codes') as string) ?? '200-299';
			host = (g('host') as string) ?? '';
			port = (g('port') as number) ?? '';
			count = (g('count') as number) ?? 3;
			recordType = (g('record_type') as string) ?? 'A';
			resolver = (g('resolver') as string) ?? '';
			warnDays = (g('warn_days') as number) ?? 30;
			criticalDays = (g('critical_days') as number) ?? 7;
			dsn = (g('dsn') as string) ?? '';
			query = (g('query') as string) ?? 'SELECT 1';
			redisAddress = (g('address') as string) ?? '';
			redisPassword = (g('password') as string) ?? '';
			redisDb = (g('db') as number) ?? 0;
			grpcAddress = (g('address') as string) ?? '';
			grpcService = (g('service') as string) ?? '';
			grpcTls = (g('tls') as boolean) ?? true;
			reopenOverrideEnabled =
				existing.reopen_window_seconds !== null || existing.reopen_mode !== null;
			reopenWindowOverride = existing.reopen_window_seconds ?? '';
			reopenModeOverride = existing.reopen_mode ?? 'flapping_only';
			advancedOpen = reopenOverrideEnabled;
		} else {
			name = '';
			type = 'http';
			intervalSeconds = 60;
			timeoutSeconds = 10;
			failureThreshold = 3;
			recoveryThreshold = 1;
			componentId = preselectComponentId;
			selectedChannelIds = [];
			url = '';
			method = 'GET';
			expectedStatusCodes = '200-299';
			host = '';
			port = '';
			count = 3;
			recordType = 'A';
			resolver = '';
			warnDays = 30;
			criticalDays = 7;
			dsn = '';
			query = 'SELECT 1';
			redisAddress = '';
			redisPassword = '';
			redisDb = 0;
			grpcAddress = '';
			grpcService = '';
			grpcTls = true;
			reopenOverrideEnabled = false;
			reopenWindowOverride = '';
			reopenModeOverride = 'flapping_only';
			advancedOpen = false;
		}
	}

	function buildConfig(): Record<string, unknown> {
		switch (type) {
			case 'http': {
				const c: Record<string, unknown> = { url, method };
				const trimmed = expectedStatusCodes.trim();
				if (trimmed && trimmed !== '200-299') c.expected_status_codes = trimmed;
				return c;
			}
			case 'tcp':
				return { host, port: Number(port) };
			case 'icmp':
				return { host, count };
			case 'dns': {
				const c: Record<string, unknown> = { host, record_type: recordType };
				if (resolver) c.resolver = resolver;
				return c;
			}
			case 'tls':
				return {
					host,
					port: port === '' ? 443 : Number(port),
					warn_days: warnDays,
					critical_days: criticalDays
				};
			case 'push':
				return {};
			case 'db_postgres':
			case 'db_mysql':
				return { dsn, query };
			case 'db_redis': {
				const c: Record<string, unknown> = { address: redisAddress, db: redisDb };
				if (redisPassword) c.password = redisPassword;
				return c;
			}
			case 'grpc': {
				const c: Record<string, unknown> = { address: grpcAddress, tls: grpcTls };
				if (grpcService) c.service = grpcService;
				return c;
			}
			default:
				return {};
		}
	}

	async function onSubmit(e: SubmitEvent) {
		e.preventDefault();
		fieldErrors = {};
		if (type === 'http' && expectedStatusCodes && !STATUS_CODES_RE.test(expectedStatusCodes)) {
			fieldErrors.expected_status_codes = $_('monitors.fields.expected_status_codes_invalid');
			return;
		}
		submitting = true;
		const payload: MonitorWriteInput = {
			name,
			interval_seconds: intervalSeconds,
			timeout_seconds: timeoutSeconds,
			failure_threshold: failureThreshold,
			recovery_threshold: recoveryThreshold,
			config: buildConfig(),
			component_id: componentId,
			notification_channel_ids: selectedChannelIds
		};
		if (reopenOverrideEnabled) {
			payload.reopen_window_seconds =
				reopenWindowOverride === '' ? null : Number(reopenWindowOverride);
			payload.reopen_mode = reopenModeOverride as ReopenMode;
		} else {
			payload.reopen_window_seconds = null;
			payload.reopen_mode = null;
		}
		if (mode === 'create') payload.type = type;
		try {
			if (mode === 'create') {
				await createMonitor(payload);
				toastSuccess($_('monitors.create.success'));
			} else if (existing) {
				await updateMonitor(existing.id, payload);
				toastSuccess($_('monitors.edit.success'));
			}
			onOpenChange(false);
			onSaved();
		} catch (err) {
			if (err instanceof ApiError && err.fields) fieldErrors = err.fields;
			toastError(err, $_('common.error_generic'));
		} finally {
			submitting = false;
		}
	}
</script>

<Dialog {open} {onOpenChange}>
	<DialogContent class="max-h-[90vh] overflow-y-auto sm:max-w-[520px]">
		<DialogHeader>
			<DialogTitle>
				{mode === 'create' ? $_('monitors.create.title') : $_('monitors.edit.title')}
			</DialogTitle>
		</DialogHeader>

		<form onsubmit={onSubmit} class="flex flex-col gap-4">
			<div class="flex flex-col gap-2">
				<Label for="m-name">{$_('monitors.fields.name')}</Label>
				<Input id="m-name" bind:value={name} required />
				{#if fieldErrors.name}<p class="text-xs text-destructive">{fieldErrorText(fieldErrors.name)}</p>{/if}
			</div>

			<div class="flex flex-col gap-2">
				<Label for="m-type">{$_('monitors.fields.type')}</Label>
				<Select.Root
					type="single"
					value={type}
					onValueChange={(v) => (type = v as MonitorType)}
					disabled={mode === 'edit'}
				>
					<Select.Trigger id="m-type" class="w-full">
						{$_(`monitors.types.${type}`)}
					</Select.Trigger>
					<Select.Content>
						{#each TYPES as t (t)}
							<Select.Item value={t}>{$_(`monitors.types.${t}`)}</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
			</div>

			<div class="flex flex-col gap-2">
				<Label for="m-component">
					{$_('monitors.fields.component')}
					<InfoTooltip text={$_('monitors.tooltips.component')} />
				</Label>
				<Select.Root
					type="single"
					value={componentId == null ? '' : String(componentId)}
					onValueChange={(v) => (componentId = v ? Number(v) : null)}
				>
					<Select.Trigger id="m-component" class="w-full">
						{componentId == null
							? $_('monitors.fields.component_none')
							: (components.find((c) => c.id === componentId)?.name ?? '')}
					</Select.Trigger>
					<Select.Content>
						<Select.Item value="">{$_('monitors.fields.component_none')}</Select.Item>
						{#each components as c (c.id)}
							<Select.Item value={String(c.id)}>{c.name}</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
			</div>

			<div class="grid grid-cols-2 gap-3">
				<div class="flex flex-col gap-2">
					<Label for="m-interval">
						{$_('monitors.fields.interval')}
						<InfoTooltip text={$_('monitors.tooltips.interval')} />
					</Label>
					<Input
						id="m-interval"
						type="number"
						min="10"
						max="86400"
						bind:value={intervalSeconds}
					/>
					{#if fieldErrors.interval_seconds}
						<p class="text-xs text-destructive">{fieldErrorText(fieldErrors.interval_seconds)}</p>
					{/if}
				</div>
				<div class="flex flex-col gap-2">
					<Label for="m-timeout">
						{$_('monitors.fields.timeout')}
						<InfoTooltip text={$_('monitors.tooltips.timeout')} />
					</Label>
					<Input
						id="m-timeout"
						type="number"
						min="1"
						max="300"
						bind:value={timeoutSeconds}
					/>
					{#if fieldErrors.timeout_seconds}
						<p class="text-xs text-destructive">{fieldErrorText(fieldErrors.timeout_seconds)}</p>
					{/if}
				</div>
			</div>

			<div class="grid grid-cols-2 gap-3">
				<div class="flex flex-col gap-2">
					<Label for="m-fthresh">
						{$_('monitors.fields.failure_threshold')}
						<InfoTooltip text={$_('monitors.tooltips.failure_threshold')} />
					</Label>
					<Input id="m-fthresh" type="number" min="1" max="20" bind:value={failureThreshold} />
				</div>
				<div class="flex flex-col gap-2">
					<Label for="m-rthresh">
						{$_('monitors.fields.recovery_threshold')}
						<InfoTooltip text={$_('monitors.tooltips.recovery_threshold')} />
					</Label>
					<Input id="m-rthresh" type="number" min="1" max="20" bind:value={recoveryThreshold} />
				</div>
			</div>

			<div class="h-px bg-border"></div>

			{#if type === 'http'}
				<div class="flex flex-col gap-2">
					<Label for="c-url">
						{$_('monitors.fields.url')}
						<InfoTooltip text={$_('monitors.tooltips.url')} />
					</Label>
					<Input id="c-url" bind:value={url} placeholder="https://example.com/health" required />
				</div>
				<div class="grid grid-cols-2 gap-3">
					<div class="flex flex-col gap-2">
						<Label for="c-method">
							{$_('monitors.fields.method')}
							<InfoTooltip text={$_('monitors.tooltips.method')} />
						</Label>
						<Select.Root type="single" bind:value={method}>
							<Select.Trigger id="c-method" class="w-full">{method}</Select.Trigger>
							<Select.Content>
								<Select.Item value="GET">GET</Select.Item>
								<Select.Item value="POST">POST</Select.Item>
								<Select.Item value="PUT">PUT</Select.Item>
								<Select.Item value="PATCH">PATCH</Select.Item>
								<Select.Item value="DELETE">DELETE</Select.Item>
								<Select.Item value="HEAD">HEAD</Select.Item>
							</Select.Content>
						</Select.Root>
					</div>
					<div class="flex flex-col gap-2">
						<Label for="c-expected">
							{$_('monitors.fields.expected_status_codes')}
							<InfoTooltip text={$_('monitors.tooltips.expected_status_codes')} />
						</Label>
						<Input id="c-expected" bind:value={expectedStatusCodes} placeholder="200-299" />
					</div>
				</div>
				<p class="text-xs text-muted-foreground">
					{$_('monitors.fields.expected_status_codes_help')}
				</p>
				{#if fieldErrors.expected_status_codes}
					<p class="text-xs text-destructive">{fieldErrorText(fieldErrors.expected_status_codes)}</p>
				{/if}
			{:else if type === 'tcp'}
				<div class="grid grid-cols-[1fr_120px] gap-3">
					<div class="flex flex-col gap-2">
						<Label for="c-host">
							{$_('monitors.fields.host')}
							<InfoTooltip text={$_('monitors.tooltips.host')} />
						</Label>
						<Input id="c-host" bind:value={host} required />
					</div>
					<div class="flex flex-col gap-2">
						<Label for="c-port">
							{$_('monitors.fields.port')}
							<InfoTooltip text={$_('monitors.tooltips.port')} />
						</Label>
						<Input id="c-port" type="number" bind:value={port} required />
					</div>
				</div>
			{:else if type === 'icmp'}
				<div class="grid grid-cols-[1fr_120px] gap-3">
					<div class="flex flex-col gap-2">
						<Label for="c-host">
							{$_('monitors.fields.host')}
							<InfoTooltip text={$_('monitors.tooltips.host')} />
						</Label>
						<Input id="c-host" bind:value={host} required />
					</div>
					<div class="flex flex-col gap-2">
						<Label for="c-count">
							{$_('monitors.fields.count')}
							<InfoTooltip text={$_('monitors.tooltips.count')} />
						</Label>
						<Input id="c-count" type="number" min="1" bind:value={count} />
					</div>
				</div>
			{:else if type === 'dns'}
				<div class="flex flex-col gap-2">
					<Label for="c-host">
						{$_('monitors.fields.host')}
						<InfoTooltip text={$_('monitors.tooltips.host')} />
					</Label>
					<Input id="c-host" bind:value={host} required />
				</div>
				<div class="grid grid-cols-2 gap-3">
					<div class="flex flex-col gap-2">
						<Label for="c-rtype">
							{$_('monitors.fields.record_type')}
							<InfoTooltip text={$_('monitors.tooltips.record_type')} />
						</Label>
						<Select.Root type="single" bind:value={recordType}>
							<Select.Trigger id="c-rtype" class="w-full">{recordType}</Select.Trigger>
							<Select.Content>
								<Select.Item value="A">A</Select.Item>
								<Select.Item value="AAAA">AAAA</Select.Item>
								<Select.Item value="CNAME">CNAME</Select.Item>
								<Select.Item value="MX">MX</Select.Item>
								<Select.Item value="TXT">TXT</Select.Item>
								<Select.Item value="NS">NS</Select.Item>
							</Select.Content>
						</Select.Root>
					</div>
					<div class="flex flex-col gap-2">
						<Label for="c-resolver">
							{$_('monitors.fields.resolver')}
							<InfoTooltip text={$_('monitors.tooltips.resolver')} />
						</Label>
						<Input id="c-resolver" bind:value={resolver} placeholder="1.1.1.1:53" />
					</div>
				</div>
			{:else if type === 'tls'}
				<div class="grid grid-cols-[1fr_120px] gap-3">
					<div class="flex flex-col gap-2">
						<Label for="c-host">
							{$_('monitors.fields.host')}
							<InfoTooltip text={$_('monitors.tooltips.host')} />
						</Label>
						<Input id="c-host" bind:value={host} required />
					</div>
					<div class="flex flex-col gap-2">
						<Label for="c-port">
							{$_('monitors.fields.port')}
							<InfoTooltip text={$_('monitors.tooltips.port')} />
						</Label>
						<Input id="c-port" type="number" bind:value={port} placeholder="443" />
					</div>
				</div>
				<div class="grid grid-cols-2 gap-3">
					<div class="flex flex-col gap-2">
						<Label for="c-warn">
							{$_('monitors.fields.warn_days')}
							<InfoTooltip text={$_('monitors.tooltips.warn_days')} />
						</Label>
						<Input id="c-warn" type="number" min="1" bind:value={warnDays} />
					</div>
					<div class="flex flex-col gap-2">
						<Label for="c-crit">
							{$_('monitors.fields.critical_days')}
							<InfoTooltip text={$_('monitors.tooltips.critical_days')} />
						</Label>
						<Input id="c-crit" type="number" min="1" bind:value={criticalDays} />
					</div>
				</div>
			{:else if type === 'push'}
				<p class="rounded-md border border-border bg-muted/40 px-3 py-2 text-xs text-muted-foreground">
					{$_('monitors.push.note')}
				</p>
			{:else if type === 'db_postgres' || type === 'db_mysql'}
				<div class="flex flex-col gap-2">
					<Label for="c-dsn">
						{$_('monitors.fields.dsn')}
						<InfoTooltip text={$_('monitors.tooltips.dsn')} />
					</Label>
					<Input id="c-dsn" type="password" bind:value={dsn} required />
				</div>
				<div class="flex flex-col gap-2">
					<Label for="c-query">
						{$_('monitors.fields.query')}
						<InfoTooltip text={$_('monitors.tooltips.query')} />
					</Label>
					<Input id="c-query" bind:value={query} />
				</div>
			{:else if type === 'db_redis'}
				<div class="flex flex-col gap-2">
					<Label for="c-addr">
						{$_('monitors.fields.address')}
						<InfoTooltip text={$_('monitors.tooltips.redis_address')} />
					</Label>
					<Input id="c-addr" bind:value={redisAddress} placeholder="localhost:6379" required />
				</div>
				<div class="grid grid-cols-[1fr_120px] gap-3">
					<div class="flex flex-col gap-2">
						<Label for="c-pw">
							{$_('monitors.fields.password')}
							<InfoTooltip text={$_('monitors.tooltips.redis_password')} />
						</Label>
						<Input id="c-pw" type="password" bind:value={redisPassword} />
					</div>
					<div class="flex flex-col gap-2">
						<Label for="c-db">
							{$_('monitors.fields.db')}
							<InfoTooltip text={$_('monitors.tooltips.redis_db')} />
						</Label>
						<Input id="c-db" type="number" min="0" bind:value={redisDb} />
					</div>
				</div>
			{:else if type === 'grpc'}
				<div class="flex flex-col gap-2">
					<Label for="c-gaddr">
						{$_('monitors.fields.address')}
						<InfoTooltip text={$_('monitors.tooltips.grpc_address')} />
					</Label>
					<Input id="c-gaddr" bind:value={grpcAddress} placeholder="host:443" required />
				</div>
				<div class="flex flex-col gap-2">
					<Label for="c-gsvc">
						{$_('monitors.fields.service')}
						<InfoTooltip text={$_('monitors.tooltips.grpc_service')} />
					</Label>
					<Input id="c-gsvc" bind:value={grpcService} />
				</div>
				<label class="flex items-center gap-2 text-sm">
					<input type="checkbox" bind:checked={grpcTls} />
					{$_('monitors.fields.tls')}
					<InfoTooltip text={$_('monitors.tooltips.grpc_tls')} />
				</label>
			{/if}

			{#if fieldErrors.config}
				<p class="text-xs text-destructive">{fieldErrorText(fieldErrors.config)}</p>
			{/if}

			<div class="h-px bg-border"></div>

			<div class="flex flex-col gap-2">
				<Label>{$_('monitors.fields.notification_channels')}</Label>
				{#if channels.length === 0}
					<p class="text-xs text-muted-foreground">
						{$_('monitors.fields.notification_channels_empty')}
					</p>
				{:else}
					<div class="flex flex-col gap-1.5 rounded-md border border-border p-2">
						{#each channels as ch (ch.id)}
							<label class="flex items-center gap-2 text-sm">
								<input
									type="checkbox"
									checked={selectedChannelIds.includes(ch.id)}
									onchange={() => toggleChannel(ch.id)}
								/>
								<span class="text-foreground">{ch.name}</span>
								<span class="text-xs text-muted-foreground">
									{$_(`notifications.types.${ch.type}`)}
								</span>
							</label>
						{/each}
					</div>
				{/if}
			</div>

			<div class="h-px bg-border"></div>

			<details class="group" bind:open={advancedOpen}>
				<summary
					class="cursor-pointer list-none text-sm text-muted-foreground hover:text-foreground"
				>
					<span class="inline-block transition-transform group-open:rotate-90">›</span>
					{$_('monitors.advanced.title')}
				</summary>
				<div class="mt-3 flex flex-col gap-3 border-l-2 border-border pl-3">
					<p class="text-xs text-muted-foreground">{$_('monitors.advanced.reopen_help')}</p>
					<label class="flex items-center gap-2 text-sm">
						<input type="checkbox" bind:checked={reopenOverrideEnabled} />
						{$_('monitors.advanced.reopen_override')}
					</label>
					{#if reopenOverrideEnabled}
						<div class="grid grid-cols-2 gap-3">
							<div class="flex flex-col gap-2">
								<Label for="m-rw">{$_('monitors.advanced.reopen_window')}</Label>
								<Input
									id="m-rw"
									type="number"
									min="0"
									max="86400"
									placeholder={$_('monitors.advanced.reopen_window_default')}
									bind:value={reopenWindowOverride}
								/>
							</div>
							<div class="flex flex-col gap-2">
								<Label for="m-rm">{$_('monitors.advanced.reopen_mode')}</Label>
								<Select.Root type="single" bind:value={reopenModeOverride}>
									<Select.Trigger id="m-rm" class="w-full">
										{$_(`settings.system.reopen_mode_${reopenModeOverride === 'flapping_only' ? 'flapping' : reopenModeOverride}`)}
									</Select.Trigger>
									<Select.Content>
										<Select.Item value="flapping_only">
											{$_('settings.system.reopen_mode_flapping')}
										</Select.Item>
										<Select.Item value="always">
											{$_('settings.system.reopen_mode_always')}
										</Select.Item>
										<Select.Item value="never">
											{$_('settings.system.reopen_mode_never')}
										</Select.Item>
									</Select.Content>
								</Select.Root>
							</div>
						</div>
					{/if}
				</div>
			</details>

			<DialogFooter>
				<Button type="button" variant="secondary" onclick={() => onOpenChange(false)}>
					{$_('common.cancel')}
				</Button>
				<Button type="submit" disabled={submitting}>
					{#if submitting}
						{$_('common.saving')}
					{:else}
						{mode === 'create' ? $_('monitors.create.submit') : $_('monitors.edit.submit')}
					{/if}
				</Button>
			</DialogFooter>
		</form>
	</DialogContent>
</Dialog>
