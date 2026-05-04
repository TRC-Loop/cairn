// SPDX-License-Identifier: AGPL-3.0-or-later
import { apiRequest } from './client';

export type MonitorType =
	| 'http'
	| 'tcp'
	| 'icmp'
	| 'dns'
	| 'tls'
	| 'push'
	| 'db_postgres'
	| 'db_mysql'
	| 'db_redis'
	| 'grpc';

export type MonitorStatus = 'up' | 'down' | 'degraded' | 'unknown' | 'paused';

export type Monitor = {
	id: number;
	name: string;
	type: MonitorType;
	enabled: boolean;
	interval_seconds: number;
	timeout_seconds: number;
	retries: number;
	failure_threshold: number;
	recovery_threshold: number;
	config: Record<string, unknown>;
	component_id: number | null;
	last_status: string;
	last_latency_ms: number | null;
	last_checked_at: string | null;
	consecutive_failures: number;
	consecutive_successes: number;
	push_token?: string;
	notification_channel_ids: number[];
	reopen_window_seconds: number | null;
	reopen_mode: ReopenMode | null;
	created_at: string;
	updated_at: string;
};

export type ReopenMode = 'always' | 'never' | 'flapping_only';

export type MonitorResult = {
	checked_at: string;
	status: string;
	latency_ms: number | null;
	error_message: string | null;
};

export type MonitorWriteInput = {
	name?: string;
	type?: MonitorType;
	enabled?: boolean;
	interval_seconds?: number;
	timeout_seconds?: number;
	retries?: number;
	failure_threshold?: number;
	recovery_threshold?: number;
	config?: Record<string, unknown>;
	component_id?: number | null;
	notification_channel_ids?: number[];
	reopen_window_seconds?: number | null;
	reopen_mode?: ReopenMode | null;
};

export async function listMonitors(): Promise<Monitor[]> {
	const { checks } = await apiRequest<{ checks: Monitor[] }>('/api/monitors');
	return checks;
}

export async function getMonitor(id: number): Promise<Monitor> {
	const { check } = await apiRequest<{ check: Monitor }>(`/api/monitors/${id}`);
	return check;
}

export async function createMonitor(input: MonitorWriteInput): Promise<Monitor> {
	const { check } = await apiRequest<{ check: Monitor }>('/api/monitors', {
		method: 'POST',
		body: input
	});
	return check;
}

export async function updateMonitor(id: number, input: MonitorWriteInput): Promise<Monitor> {
	const { check } = await apiRequest<{ check: Monitor }>(`/api/monitors/${id}`, {
		method: 'PATCH',
		body: input
	});
	return check;
}

export async function deleteMonitor(id: number): Promise<void> {
	await apiRequest(`/api/monitors/${id}`, { method: 'DELETE' });
}

export async function getMonitorResults(id: number, hours = 24): Promise<MonitorResult[]> {
	const { results } = await apiRequest<{ results: MonitorResult[] }>(
		`/api/monitors/${id}/results?hours=${hours}`
	);
	return results;
}
