// SPDX-License-Identifier: AGPL-3.0-or-later
import { apiRequest } from './client';
import type { Monitor } from './monitors';

export type IncidentStatus = 'investigating' | 'identified' | 'monitoring' | 'resolved';
export type IncidentSeverity = 'minor' | 'major' | 'critical';

export type CheckSummary = {
	id: number;
	name: string;
	type: string;
	last_status: string;
};

export type Incident = {
	id: number;
	display_id: string;
	title: string;
	status: IncidentStatus;
	severity: IncidentSeverity;
	started_at: string;
	resolved_at: string | null;
	auto_created: boolean;
	triggering_check_id: number | null;
	triggering_check: CheckSummary | null;
	created_by_user_id: number | null;
	affected_check_count: number;
	created_at: string;
	updated_at: string;
};

export type IncidentUpdate = {
	id: number;
	incident_id: number;
	status: IncidentStatus;
	message: string;
	message_html: string;
	posted_by_user_id: number | null;
	auto_generated: boolean;
	created_at: string;
};

export type IncidentDetail = {
	incident: Incident;
	updates: IncidentUpdate[];
	affected_checks: Monitor[];
};

export type IncidentCreateInput = {
	title: string;
	severity: IncidentSeverity;
	initial_message: string;
	affected_check_ids: number[];
};

export type IncidentPatchInput = {
	title?: string;
	severity?: IncidentSeverity;
};

export type IncidentAddUpdateInput = {
	message: string;
	new_status?: IncidentStatus;
};

export type IncidentListFilter = 'all' | 'active' | 'resolved';

export async function listIncidents(
	status: IncidentListFilter = 'all'
): Promise<{ incidents: Incident[]; total: number }> {
	return apiRequest<{ incidents: Incident[]; total: number }>(
		`/api/incidents?status=${status}&limit=200`
	);
}

export async function getIncident(id: number): Promise<IncidentDetail> {
	return apiRequest<IncidentDetail>(`/api/incidents/${id}`);
}

export async function createIncident(
	input: IncidentCreateInput
): Promise<{ incident: Incident; updates: IncidentUpdate[] }> {
	return apiRequest<{ incident: Incident; updates: IncidentUpdate[] }>('/api/incidents', {
		method: 'POST',
		body: input
	});
}

export async function patchIncident(id: number, input: IncidentPatchInput): Promise<Incident> {
	const { incident } = await apiRequest<{ incident: Incident }>(`/api/incidents/${id}`, {
		method: 'PATCH',
		body: input
	});
	return incident;
}

export async function addIncidentUpdate(
	id: number,
	input: IncidentAddUpdateInput
): Promise<{ update: IncidentUpdate; incident: Incident }> {
	return apiRequest<{ update: IncidentUpdate; incident: Incident }>(
		`/api/incidents/${id}/updates`,
		{ method: 'POST', body: input }
	);
}

export async function addAffectedCheck(id: number, checkId: number): Promise<void> {
	await apiRequest(`/api/incidents/${id}/affected-checks`, {
		method: 'POST',
		body: { check_id: checkId }
	});
}

export async function removeAffectedCheck(id: number, checkId: number): Promise<void> {
	await apiRequest(`/api/incidents/${id}/affected-checks/${checkId}`, { method: 'DELETE' });
}

export async function deleteIncident(id: number): Promise<void> {
	await apiRequest(`/api/incidents/${id}`, { method: 'DELETE' });
}
