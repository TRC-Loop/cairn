// SPDX-License-Identifier: AGPL-3.0-or-later
import { apiRequest } from './client';

export type Role = 'admin' | 'editor' | 'viewer';

export type UserRecord = {
	id: number;
	username: string;
	email: string;
	display_name: string;
	role: Role;
	created_at: string;
	updated_at: string;
	totp_enabled?: boolean;
};

export type CreateUserInput = {
	username: string;
	email: string;
	display_name: string;
	password: string;
	role: Role;
};

export type UpdateUserInput = {
	email?: string;
	display_name?: string;
	role?: Role;
};

export type ChangePasswordInput = {
	current_password?: string;
	new_password: string;
	force_logout?: boolean;
};

export async function listUsers(): Promise<UserRecord[]> {
	const { users } = await apiRequest<{ users: UserRecord[]; total: number }>('/api/users');
	return users;
}

export async function getUser(id: number): Promise<UserRecord> {
	const { user } = await apiRequest<{ user: UserRecord }>(`/api/users/${id}`);
	return user;
}

export async function createUser(input: CreateUserInput): Promise<UserRecord> {
	const { user } = await apiRequest<{ user: UserRecord }>('/api/users', {
		method: 'POST',
		body: input
	});
	return user;
}

export async function updateUser(id: number, input: UpdateUserInput): Promise<UserRecord> {
	const { user } = await apiRequest<{ user: UserRecord }>(`/api/users/${id}`, {
		method: 'PATCH',
		body: input
	});
	return user;
}

export async function changePassword(id: number, input: ChangePasswordInput): Promise<void> {
	await apiRequest(`/api/users/${id}/password`, { method: 'POST', body: input });
}

export async function deleteUser(id: number): Promise<void> {
	await apiRequest(`/api/users/${id}`, { method: 'DELETE' });
}
