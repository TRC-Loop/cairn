// SPDX-License-Identifier: AGPL-3.0-or-later
import { apiRequest } from './client';
import type { Component } from './components';

export type MaintenanceState = 'scheduled' | 'in_progress' | 'completed' | 'cancelled';

export type MaintenanceWindow = {
	id: number;
	title: string;
	description: string;
	description_html: string;
	starts_at: string;
	ends_at: string;
	state: MaintenanceState;
	created_by_user_id: number | null;
	created_at: string;
	updated_at: string;
	affected_component_ids: number[];
	affected_component_names: string[];
};

export type MaintenanceDetail = {
	window: MaintenanceWindow;
	affected_components: Component[];
};

export type MaintenanceListFilter = 'all' | MaintenanceState;

export type MaintenanceCreateInput = {
	title: string;
	description?: string;
	starts_at: string;
	ends_at: string;
	affected_component_ids: number[];
};

export type MaintenancePatchInput = {
	title?: string;
	description?: string;
	starts_at?: string;
	ends_at?: string;
	affected_component_ids?: number[];
};

export async function listMaintenance(
	params: { status?: MaintenanceListFilter; upcoming?: boolean; pastDays?: number; limit?: number } = {}
): Promise<{ maintenance: MaintenanceWindow[]; total: number }> {
	const qs = new URLSearchParams();
	if (params.status && params.status !== 'all') qs.set('status', params.status);
	if (params.upcoming) qs.set('upcoming', '1');
	if (params.pastDays) qs.set('past_days', String(params.pastDays));
	if (params.limit) qs.set('limit', String(params.limit));
	const q = qs.toString();
	return apiRequest<{ maintenance: MaintenanceWindow[]; total: number }>(
		`/api/maintenance${q ? `?${q}` : ''}`
	);
}

export async function getMaintenance(id: number): Promise<MaintenanceDetail> {
	return apiRequest<MaintenanceDetail>(`/api/maintenance/${id}`);
}

export async function createMaintenance(
	input: MaintenanceCreateInput
): Promise<MaintenanceWindow> {
	const { window } = await apiRequest<{ window: MaintenanceWindow }>('/api/maintenance', {
		method: 'POST',
		body: input
	});
	return window;
}

export async function patchMaintenance(
	id: number,
	input: MaintenancePatchInput
): Promise<MaintenanceWindow> {
	const { window } = await apiRequest<{ window: MaintenanceWindow }>(`/api/maintenance/${id}`, {
		method: 'PATCH',
		body: input
	});
	return window;
}

export async function cancelMaintenance(id: number): Promise<MaintenanceWindow> {
	const { window } = await apiRequest<{ window: MaintenanceWindow }>(
		`/api/maintenance/${id}/cancel`,
		{ method: 'POST' }
	);
	return window;
}

export async function endMaintenanceNow(id: number): Promise<MaintenanceWindow> {
	const { window } = await apiRequest<{ window: MaintenanceWindow }>(
		`/api/maintenance/${id}/end-now`,
		{ method: 'POST' }
	);
	return window;
}

export async function deleteMaintenance(id: number): Promise<void> {
	await apiRequest(`/api/maintenance/${id}`, { method: 'DELETE' });
}
