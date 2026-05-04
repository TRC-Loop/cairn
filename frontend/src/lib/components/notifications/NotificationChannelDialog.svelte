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
	import {
		createChannel,
		updateChannel,
		type ChannelType,
		type NotificationChannel,
		type NotificationChannelWriteInput
	} from '$lib/api/notifications';
	import { ApiError } from '$lib/api/client';
	import { toastError, toastSuccess } from '$lib/toast';
	import { fieldErrorText } from '$lib/validate';

	type Props = {
		open: boolean;
		mode: 'create' | 'edit';
		existing?: NotificationChannel | null;
		onOpenChange: (v: boolean) => void;
		onSaved: () => void;
	};

	let { open, mode, existing = null, onOpenChange, onSaved }: Props = $props();

	const TYPES: ChannelType[] = ['email', 'discord', 'webhook'];

	let name = $state('');
	let type = $state<ChannelType>('email');
	let enabled = $state(true);
	let retryMax = $state(3);
	let retryBackoff = $state(1);

	// email
	let smtpHost = $state('');
	let smtpPort = $state(587);
	let smtpStartTLS = $state(true);
	let smtpUsername = $state('');
	let smtpPassword = $state('');
	let smtpPasswordSet = $state(false);
	let fromAddress = $state('');
	let fromName = $state('Cairn');
	let toAddressesText = $state('');

	// discord
	let webhookURL = $state('');
	let webhookURLSet = $state(false);
	let discordUsername = $state('Cairn');
	let avatarURL = $state('');

	// webhook
	let whURL = $state('');
	let whMethod = $state<'POST' | 'PUT'>('POST');
	let whSecret = $state('');
	let whSecretSet = $state(false);
	let whHeadersText = $state('');

	let saving = $state(false);
	let fieldErrors = $state<Record<string, string>>({});

	$effect(() => {
		if (open) {
			initFromExisting();
			fieldErrors = {};
		}
	});

	function g<T = unknown>(cfg: Record<string, unknown> | undefined, key: string): T | undefined {
		if (!cfg) return undefined;
		return cfg[key] as T | undefined;
	}

	function initFromExisting() {
		if (mode === 'edit' && existing) {
			name = existing.name;
			type = existing.type;
			enabled = existing.enabled;
			retryMax = existing.retry_max;
			retryBackoff = existing.retry_backoff_seconds;
			const cfg = existing.config ?? {};
			smtpHost = g<string>(cfg, 'smtp_host') ?? '';
			smtpPort = g<number>(cfg, 'smtp_port') ?? 587;
			smtpStartTLS = g<boolean>(cfg, 'smtp_starttls') ?? true;
			smtpUsername = g<string>(cfg, 'smtp_username') ?? '';
			smtpPassword = '';
			smtpPasswordSet = g<boolean>(cfg, 'smtp_password_set') ?? false;
			fromAddress = g<string>(cfg, 'from_address') ?? '';
			fromName = g<string>(cfg, 'from_name') ?? 'Cairn';
			toAddressesText = (g<string[]>(cfg, 'to_addresses') ?? []).join(', ');
			webhookURL = '';
			webhookURLSet = g<boolean>(cfg, 'webhook_url_set') ?? false;
			discordUsername = g<string>(cfg, 'username') ?? 'Cairn';
			avatarURL = g<string>(cfg, 'avatar_url') ?? '';
			whURL = g<string>(cfg, 'url') ?? '';
			whMethod = (g<'POST' | 'PUT'>(cfg, 'method') ?? 'POST') as 'POST' | 'PUT';
			whSecret = '';
			whSecretSet = g<boolean>(cfg, 'secret_set') ?? false;
			const headers = g<Record<string, string>>(cfg, 'extra_headers') ?? {};
			whHeadersText = Object.entries(headers)
				.map(([k, v]) => `${k}: ${v}`)
				.join('\n');
		} else {
			name = '';
			type = 'email';
			enabled = true;
			retryMax = 3;
			retryBackoff = 1;
			smtpHost = '';
			smtpPort = 587;
			smtpStartTLS = true;
			smtpUsername = '';
			smtpPassword = '';
			smtpPasswordSet = false;
			fromAddress = '';
			fromName = 'Cairn';
			toAddressesText = '';
			webhookURL = '';
			webhookURLSet = false;
			discordUsername = 'Cairn';
			avatarURL = '';
			whURL = '';
			whMethod = 'POST';
			whSecret = '';
			whSecretSet = false;
			whHeadersText = '';
		}
	}

	function parseHeaders(text: string): Record<string, string> {
		const out: Record<string, string> = {};
		for (const line of text.split('\n')) {
			const trimmed = line.trim();
			if (!trimmed) continue;
			const idx = trimmed.indexOf(':');
			if (idx <= 0) continue;
			const k = trimmed.slice(0, idx).trim();
			const v = trimmed.slice(idx + 1).trim();
			if (k) out[k] = v;
		}
		return out;
	}

	function buildConfig(): Record<string, unknown> {
		switch (type) {
			case 'email': {
				const cfg: Record<string, unknown> = {
					smtp_host: smtpHost,
					smtp_port: smtpPort,
					smtp_starttls: smtpStartTLS,
					smtp_username: smtpUsername,
					from_address: fromAddress,
					from_name: fromName,
					to_addresses: toAddressesText
						.split(/[,\n]/)
						.map((s) => s.trim())
						.filter(Boolean)
				};
				if (smtpPassword) cfg.smtp_password = smtpPassword;
				return cfg;
			}
			case 'discord': {
				const cfg: Record<string, unknown> = {
					username: discordUsername,
					avatar_url: avatarURL
				};
				if (webhookURL) cfg.webhook_url = webhookURL;
				return cfg;
			}
			case 'webhook': {
				const cfg: Record<string, unknown> = {
					url: whURL,
					method: whMethod,
					extra_headers: parseHeaders(whHeadersText)
				};
				if (whSecret) cfg.secret = whSecret;
				return cfg;
			}
		}
	}

	async function onSubmit(e: SubmitEvent) {
		e.preventDefault();
		fieldErrors = {};
		saving = true;
		const payload: NotificationChannelWriteInput = {
			name,
			enabled,
			retry_max: retryMax,
			retry_backoff_seconds: retryBackoff,
			config: buildConfig()
		};
		if (mode === 'create') payload.type = type;
		try {
			if (mode === 'create') {
				await createChannel(payload);
				toastSuccess($_('notifications.create.success'));
			} else if (existing) {
				await updateChannel(existing.id, payload);
				toastSuccess($_('notifications.edit.success'));
			}
			onOpenChange(false);
			onSaved();
		} catch (err) {
			if (err instanceof ApiError && err.fields) fieldErrors = err.fields;
			toastError(err, $_('common.error_generic'));
		} finally {
			saving = false;
		}
	}
</script>

<Dialog {open} {onOpenChange}>
	<DialogContent class="max-h-[90vh] overflow-y-auto sm:max-w-[560px]">
		<DialogHeader>
			<DialogTitle>
				{mode === 'create'
					? $_('notifications.create.title')
					: $_('notifications.edit.title')}
			</DialogTitle>
		</DialogHeader>

		<form onsubmit={onSubmit} class="flex flex-col gap-4">
			<div class="flex flex-col gap-2">
				<Label for="ch-name">{$_('notifications.fields.name')}</Label>
				<Input id="ch-name" bind:value={name} required maxlength={100} />
				{#if fieldErrors.name}<p class="text-xs text-destructive">{fieldErrorText(fieldErrors.name)}</p>{/if}
			</div>

			<div class="flex flex-col gap-2">
				<Label for="ch-type">{$_('notifications.fields.type')}</Label>
				<Select.Root
					type="single"
					value={type}
					onValueChange={(v) => (type = v as ChannelType)}
					disabled={mode === 'edit'}
				>
					<Select.Trigger id="ch-type" class="w-full">
						{$_(`notifications.types.${type}`)}
					</Select.Trigger>
					<Select.Content>
						{#each TYPES as t (t)}
							<Select.Item value={t}>{$_(`notifications.types.${t}`)}</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
			</div>

			<label class="flex items-center gap-2 text-sm">
				<input type="checkbox" bind:checked={enabled} />
				{$_('notifications.fields.enabled')}
			</label>

			<div class="grid grid-cols-2 gap-3">
				<div class="flex flex-col gap-2">
					<Label for="ch-retry">{$_('notifications.fields.retry_max')}</Label>
					<Input id="ch-retry" type="number" min="0" max="10" bind:value={retryMax} />
					{#if fieldErrors.retry_max}
						<p class="text-xs text-destructive">{fieldErrorText(fieldErrors.retry_max)}</p>
					{/if}
				</div>
				<div class="flex flex-col gap-2">
					<Label for="ch-backoff">{$_('notifications.fields.retry_backoff_seconds')}</Label>
					<Input id="ch-backoff" type="number" min="1" bind:value={retryBackoff} />
					{#if fieldErrors.retry_backoff_seconds}
						<p class="text-xs text-destructive">{fieldErrorText(fieldErrors.retry_backoff_seconds)}</p>
					{/if}
				</div>
			</div>

			<div class="h-px bg-border"></div>

			{#if type === 'email'}
				<div class="grid grid-cols-[1fr_140px] gap-3">
					<div class="flex flex-col gap-2">
						<Label for="ch-smtp-host">{$_('notifications.fields.smtp_host')}</Label>
						<Input id="ch-smtp-host" bind:value={smtpHost} required />
						{#if fieldErrors.smtp_host}
							<p class="text-xs text-destructive">{fieldErrorText(fieldErrors.smtp_host)}</p>
						{/if}
					</div>
					<div class="flex flex-col gap-2">
						<Label for="ch-smtp-port">{$_('notifications.fields.smtp_port')}</Label>
						<Input id="ch-smtp-port" type="number" bind:value={smtpPort} />
						{#if fieldErrors.smtp_port}
							<p class="text-xs text-destructive">{fieldErrorText(fieldErrors.smtp_port)}</p>
						{/if}
					</div>
				</div>
				<label class="flex items-center gap-2 text-sm">
					<input type="checkbox" bind:checked={smtpStartTLS} />
					{$_('notifications.fields.smtp_starttls')}
				</label>
				<div class="grid grid-cols-2 gap-3">
					<div class="flex flex-col gap-2">
						<Label for="ch-smtp-user">{$_('notifications.fields.smtp_username')}</Label>
						<Input id="ch-smtp-user" bind:value={smtpUsername} />
					</div>
					<div class="flex flex-col gap-2">
						<Label for="ch-smtp-pw">{$_('notifications.fields.smtp_password')}</Label>
						<Input
							id="ch-smtp-pw"
							type="password"
							bind:value={smtpPassword}
							placeholder={smtpPasswordSet ? $_('notifications.fields.password_kept') : ''}
						/>
					</div>
				</div>
				<div class="grid grid-cols-2 gap-3">
					<div class="flex flex-col gap-2">
						<Label for="ch-from-addr">{$_('notifications.fields.from_address')}</Label>
						<Input id="ch-from-addr" bind:value={fromAddress} required />
						{#if fieldErrors.from_address}
							<p class="text-xs text-destructive">{fieldErrorText(fieldErrors.from_address)}</p>
						{/if}
					</div>
					<div class="flex flex-col gap-2">
						<Label for="ch-from-name">{$_('notifications.fields.from_name')}</Label>
						<Input id="ch-from-name" bind:value={fromName} />
					</div>
				</div>
				<div class="flex flex-col gap-2">
					<Label for="ch-to">{$_('notifications.fields.to_addresses')}</Label>
					<Input id="ch-to" bind:value={toAddressesText} placeholder="alice@example.com, bob@example.com" />
					<p class="text-xs text-muted-foreground">{$_('notifications.fields.to_addresses_help')}</p>
					{#if fieldErrors.to_addresses}
						<p class="text-xs text-destructive">{fieldErrorText(fieldErrors.to_addresses)}</p>
					{/if}
				</div>
			{:else if type === 'discord'}
				<div class="flex flex-col gap-2">
					<Label for="ch-wh-url">{$_('notifications.fields.webhook_url')}</Label>
					<Input
						id="ch-wh-url"
						type="password"
						bind:value={webhookURL}
						placeholder={webhookURLSet ? $_('notifications.fields.password_kept') : ''}
					/>
					{#if fieldErrors.webhook_url}
						<p class="text-xs text-destructive">{fieldErrorText(fieldErrors.webhook_url)}</p>
					{/if}
				</div>
				<div class="grid grid-cols-2 gap-3">
					<div class="flex flex-col gap-2">
						<Label for="ch-d-user">{$_('notifications.fields.discord_username')}</Label>
						<Input id="ch-d-user" bind:value={discordUsername} />
					</div>
					<div class="flex flex-col gap-2">
						<Label for="ch-d-avatar">{$_('notifications.fields.avatar_url')}</Label>
						<Input id="ch-d-avatar" bind:value={avatarURL} />
					</div>
				</div>
			{:else if type === 'webhook'}
				<div class="grid grid-cols-[1fr_120px] gap-3">
					<div class="flex flex-col gap-2">
						<Label for="ch-w-url">{$_('notifications.fields.webhook_target_url')}</Label>
						<Input id="ch-w-url" bind:value={whURL} required />
						{#if fieldErrors.url}<p class="text-xs text-destructive">{fieldErrorText(fieldErrors.url)}</p>{/if}
					</div>
					<div class="flex flex-col gap-2">
						<Label for="ch-w-method">{$_('notifications.fields.method')}</Label>
						<Select.Root type="single" bind:value={whMethod}>
							<Select.Trigger id="ch-w-method" class="w-full">{whMethod}</Select.Trigger>
							<Select.Content>
								<Select.Item value="POST">POST</Select.Item>
								<Select.Item value="PUT">PUT</Select.Item>
							</Select.Content>
						</Select.Root>
					</div>
				</div>
				<div class="flex flex-col gap-2">
					<Label for="ch-w-secret">{$_('notifications.fields.signing_secret')}</Label>
					<Input
						id="ch-w-secret"
						type="password"
						bind:value={whSecret}
						placeholder={whSecretSet ? $_('notifications.fields.password_kept') : ''}
					/>
					<p class="text-xs text-muted-foreground">{$_('notifications.fields.signing_secret_help')}</p>
				</div>
				<div class="flex flex-col gap-2">
					<Label for="ch-w-headers">{$_('notifications.fields.extra_headers')}</Label>
					<textarea
						id="ch-w-headers"
						bind:value={whHeadersText}
						rows="3"
						class="border-input bg-background placeholder:text-muted-foreground focus-visible:ring-ring flex w-full rounded-md border px-3 py-2 text-sm focus-visible:ring-1 focus-visible:outline-none"
						placeholder="X-Source: cairn"
					></textarea>
					<p class="text-xs text-muted-foreground">
						{$_('notifications.fields.extra_headers_help')}
					</p>
				</div>
			{/if}

			{#if fieldErrors.config}
				<p class="text-xs text-destructive">{fieldErrorText(fieldErrors.config)}</p>
			{/if}

			<DialogFooter>
				<Button type="button" variant="secondary" onclick={() => onOpenChange(false)}>
					{$_('common.cancel')}
				</Button>
				<Button type="submit" disabled={saving}>
					{#if saving}
						{$_('common.saving')}
					{:else}
						{mode === 'create'
							? $_('notifications.create.submit')
							: $_('notifications.edit.submit')}
					{/if}
				</Button>
			</DialogFooter>
		</form>
	</DialogContent>
</Dialog>
