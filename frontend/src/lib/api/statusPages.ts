// SPDX-License-Identifier: AGPL-3.0-or-later
import { apiRequest } from './client';
import type { Component } from './components';

export type StatusPage = {
	id: number;
	slug: string;
	title: string;
	description: string;
	logo_url: string;
	accent_color: string;
	custom_footer_html: string;
	footer_mode: FooterMode;
	password_set: boolean;
	is_default: boolean;
	hide_powered_by: boolean;
	show_history: boolean;
	created_at: string;
	updated_at: string;
};

export type StatusPageDomain = {
	id: number;
	domain: string;
	created_at: string;
};

export type FooterMode = 'structured' | 'html' | 'both';
export type FooterElementType = 'link' | 'text' | 'separator';

export type FooterElement = {
	id: number;
	element_type: FooterElementType;
	label?: string;
	url?: string;
	open_in_new_tab: boolean;
	display_order: number;
};

export type FooterElementInput = {
	element_type: FooterElementType;
	label?: string;
	url?: string;
	open_in_new_tab?: boolean;
};

export type FooterPayload = {
	footer_mode: FooterMode;
	elements: FooterElement[];
	custom_footer_html: string;
};

export type StatusPageWriteInput = {
	slug?: string;
	title?: string;
	description?: string;
	logo_url?: string;
	accent_color?: string;
	custom_footer_html?: string;
	is_default?: boolean;
	hide_powered_by?: boolean;
	show_history?: boolean;
};

export async function listStatusPages(): Promise<StatusPage[]> {
	const { status_pages } = await apiRequest<{ status_pages: StatusPage[] }>('/api/status-pages');
	return status_pages;
}

export async function getStatusPage(
	id: number
): Promise<{ status_page: StatusPage; components: Component[] }> {
	return apiRequest<{ status_page: StatusPage; components: Component[] }>(
		`/api/status-pages/${id}`
	);
}

export async function createStatusPage(input: StatusPageWriteInput): Promise<StatusPage> {
	const { status_page } = await apiRequest<{ status_page: StatusPage }>('/api/status-pages', {
		method: 'POST',
		body: input
	});
	return status_page;
}

export async function updateStatusPage(
	id: number,
	input: StatusPageWriteInput
): Promise<StatusPage> {
	const { status_page } = await apiRequest<{ status_page: StatusPage }>(
		`/api/status-pages/${id}`,
		{ method: 'PATCH', body: input }
	);
	return status_page;
}

export async function deleteStatusPage(id: number): Promise<void> {
	await apiRequest(`/api/status-pages/${id}`, { method: 'DELETE' });
}

export async function setDefaultStatusPage(id: number): Promise<void> {
	await apiRequest(`/api/status-pages/${id}/default`, { method: 'POST' });
}

export async function setStatusPagePassword(id: number, password: string): Promise<void> {
	await apiRequest(`/api/status-pages/${id}/password`, {
		method: 'POST',
		body: { password }
	});
}

export async function setStatusPageComponents(id: number, componentIds: number[]): Promise<void> {
	await apiRequest(`/api/status-pages/${id}/components`, {
		method: 'PUT',
		body: { component_ids: componentIds }
	});
}

export async function getStatusPageFooter(id: number): Promise<FooterPayload> {
	return apiRequest<FooterPayload>(`/api/status-pages/${id}/footer`);
}

export async function replaceFooterElements(
	id: number,
	elements: FooterElementInput[]
): Promise<FooterElement[]> {
	const { elements: out } = await apiRequest<{ elements: FooterElement[] }>(
		`/api/status-pages/${id}/footer/elements`,
		{ method: 'PUT', body: { elements } }
	);
	return out;
}

export async function setStatusPageFooterMode(id: number, mode: FooterMode): Promise<void> {
	await apiRequest(`/api/status-pages/${id}/footer/mode`, {
		method: 'PUT',
		body: { footer_mode: mode }
	});
}

export async function listStatusPageDomains(id: number): Promise<StatusPageDomain[]> {
	const { domains } = await apiRequest<{ domains: StatusPageDomain[] }>(
		`/api/status-pages/${id}/domains`
	);
	return domains;
}

export async function addStatusPageDomain(
	id: number,
	domain: string
): Promise<StatusPageDomain> {
	const { domain: row } = await apiRequest<{ domain: StatusPageDomain }>(
		`/api/status-pages/${id}/domains`,
		{ method: 'POST', body: { domain } }
	);
	return row;
}

export async function deleteStatusPageDomain(id: number, domainId: number): Promise<void> {
	await apiRequest(`/api/status-pages/${id}/domains/${domainId}`, { method: 'DELETE' });
}

export type DirectMonitor = { id: number; name: string; last_status: string };

export async function listStatusPageMonitors(id: number): Promise<DirectMonitor[]> {
	const { monitors } = await apiRequest<{ monitors: DirectMonitor[] }>(
		`/api/status-pages/${id}/monitors`
	);
	return monitors;
}

export async function setStatusPageMonitors(id: number, monitorIds: number[]): Promise<void> {
	await apiRequest(`/api/status-pages/${id}/monitors`, {
		method: 'PUT',
		body: { monitor_ids: monitorIds }
	});
}
