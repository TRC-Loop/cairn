// SPDX-License-Identifier: AGPL-3.0-or-later
import { apiRequest } from './client';

export type RetentionSettings = {
	raw_days: number;
	hourly_days: number;
	daily_days: number;
	keep_daily_forever: boolean;
	updated_at: string;
};

export type RetentionSettingsInput = Partial<Omit<RetentionSettings, 'updated_at'>>;

export async function getRetentionSettings(): Promise<RetentionSettings> {
	const { settings } = await apiRequest<{ settings: RetentionSettings }>('/api/retention-settings');
	return settings;
}

export async function updateRetentionSettings(
	input: RetentionSettingsInput
): Promise<RetentionSettings> {
	const { settings } = await apiRequest<{ settings: RetentionSettings }>('/api/retention-settings', {
		method: 'PATCH',
		body: input
	});
	return settings;
}
