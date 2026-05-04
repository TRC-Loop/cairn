// SPDX-License-Identifier: AGPL-3.0-or-later
import { apiRequest } from './client';
import type { Monitor } from './monitors';

export type Component = {
	id: number;
	name: string;
	description: string | null;
	display_order: number;
	check_count?: number;
	created_at: string;
	updated_at: string;
};

export type ComponentWriteInput = {
	name?: string;
	description?: string;
	display_order?: number;
};

export async function listComponents(): Promise<Component[]> {
	const { components } = await apiRequest<{ components: Component[] }>('/api/components');
	return components;
}

export async function getComponent(id: number): Promise<{ component: Component; checks: Monitor[] }> {
	return apiRequest<{ component: Component; checks: Monitor[] }>(`/api/components/${id}`);
}

export async function createComponent(input: ComponentWriteInput): Promise<Component> {
	const { component } = await apiRequest<{ component: Component }>('/api/components', {
		method: 'POST',
		body: input
	});
	return component;
}

export async function updateComponent(id: number, input: ComponentWriteInput): Promise<Component> {
	const { component } = await apiRequest<{ component: Component }>(`/api/components/${id}`, {
		method: 'PATCH',
		body: input
	});
	return component;
}

export async function deleteComponent(id: number): Promise<void> {
	await apiRequest(`/api/components/${id}`, { method: 'DELETE' });
}
