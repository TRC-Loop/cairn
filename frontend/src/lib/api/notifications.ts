// SPDX-License-Identifier: AGPL-3.0-or-later
import { apiRequest } from './client';

export type ChannelType = 'email' | 'discord' | 'webhook';

export type EmailConfig = {
	smtp_host?: string;
	smtp_port?: number;
	smtp_starttls?: boolean;
	smtp_username?: string;
	smtp_password?: string;
	smtp_password_set?: boolean;
	from_address?: string;
	from_name?: string;
	to_addresses?: string[];
};

export type DiscordConfig = {
	webhook_url?: string;
	webhook_url_set?: boolean;
	username?: string;
	avatar_url?: string;
};

export type WebhookConfig = {
	url?: string;
	method?: 'POST' | 'PUT';
	extra_headers?: Record<string, string>;
	secret?: string;
	secret_set?: boolean;
};

export type ChannelConfig = EmailConfig | DiscordConfig | WebhookConfig;

export type NotificationChannel = {
	id: number;
	name: string;
	type: ChannelType;
	enabled: boolean;
	retry_max: number;
	retry_backoff_seconds: number;
	config: Record<string, unknown>;
	used_by_check_count: number;
	created_at: string;
	updated_at: string;
};

export type NotificationChannelWriteInput = {
	name?: string;
	type?: ChannelType;
	enabled?: boolean;
	retry_max?: number;
	retry_backoff_seconds?: number;
	config?: Record<string, unknown>;
};

export type NotificationDelivery = {
	id: number;
	channel_id: number;
	event_type: string;
	event_id: number;
	status: 'pending' | 'sending' | 'sent' | 'failed';
	attempt_count: number;
	last_error: string | null;
	last_attempted_at: string | null;
	next_attempt_at: string | null;
	sent_at: string | null;
	created_at: string;
};

export async function listChannels(): Promise<NotificationChannel[]> {
	const { channels } = await apiRequest<{ channels: NotificationChannel[] }>(
		'/api/notification-channels'
	);
	return channels;
}

export async function getChannel(id: number): Promise<NotificationChannel> {
	const { channel } = await apiRequest<{ channel: NotificationChannel }>(
		`/api/notification-channels/${id}`
	);
	return channel;
}

export async function createChannel(
	input: NotificationChannelWriteInput
): Promise<NotificationChannel> {
	const { channel } = await apiRequest<{ channel: NotificationChannel }>(
		'/api/notification-channels',
		{ method: 'POST', body: input }
	);
	return channel;
}

export async function updateChannel(
	id: number,
	input: NotificationChannelWriteInput
): Promise<NotificationChannel> {
	const { channel } = await apiRequest<{ channel: NotificationChannel }>(
		`/api/notification-channels/${id}`,
		{ method: 'PATCH', body: input }
	);
	return channel;
}

export async function deleteChannel(id: number): Promise<void> {
	await apiRequest(`/api/notification-channels/${id}`, { method: 'DELETE' });
}

export async function testChannel(id: number): Promise<{ delivery_id: number; status: string }> {
	return apiRequest<{ delivery_id: number; status: string }>(
		`/api/notification-channels/${id}/test`,
		{ method: 'POST' }
	);
}

export async function listDeliveries(channelID: number): Promise<NotificationDelivery[]> {
	const { deliveries } = await apiRequest<{ deliveries: NotificationDelivery[] }>(
		`/api/notification-channels/${channelID}/deliveries`
	);
	return deliveries;
}

export async function getDelivery(id: number): Promise<NotificationDelivery> {
	const { delivery } = await apiRequest<{ delivery: NotificationDelivery }>(
		`/api/notification-deliveries/${id}`
	);
	return delivery;
}
