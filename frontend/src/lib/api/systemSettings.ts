// SPDX-License-Identifier: AGPL-3.0-or-later
import { apiRequest } from './client';

export type ReopenMode = 'always' | 'never' | 'flapping_only';

export type SystemSettings = {
	incident_id_format: string;
	incident_reopen_window_seconds: number;
	incident_reopen_mode: ReopenMode;
	updated_at: string;
};

export type SystemSettingsInput = {
	incident_id_format?: string;
	incident_reopen_window_seconds?: number;
	incident_reopen_mode?: ReopenMode;
};

export async function getSystemSettings(): Promise<SystemSettings> {
	const { settings } = await apiRequest<{ settings: SystemSettings }>('/api/system-settings');
	return settings;
}

export async function updateSystemSettings(input: SystemSettingsInput): Promise<SystemSettings> {
	const { settings } = await apiRequest<{ settings: SystemSettings }>('/api/system-settings', {
		method: 'PATCH',
		body: input
	});
	return settings;
}
