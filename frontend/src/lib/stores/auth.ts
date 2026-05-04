// SPDX-License-Identifier: AGPL-3.0-or-later
import { writable } from 'svelte/store';
import { apiRequest, ApiError } from '$lib/api/client';

export type User = {
	id: number;
	username: string;
	email: string;
	display_name: string;
	role: 'admin' | 'editor' | 'viewer';
	totp_enabled?: boolean;
};

type AuthState = {
	user: User | null;
	loading: boolean;
	initialized: boolean;
};

function createAuthStore() {
	const { subscribe, set, update } = writable<AuthState>({
		user: null,
		loading: false,
		initialized: false
	});

	return {
		subscribe,

		async init() {
			update((s) => ({ ...s, loading: true }));
			try {
				const { user } = await apiRequest<{ user: User }>('/api/auth/me');
				set({ user, loading: false, initialized: true });
			} catch (err) {
				if (err instanceof ApiError && err.status === 401) {
					set({ user: null, loading: false, initialized: true });
				} else {
					update((s) => ({ ...s, loading: false }));
					throw err;
				}
			}
		},

		async login(
			identifier: string,
			password: string
		): Promise<{ kind: 'session'; user: User } | { kind: 'challenge'; challenge_token: string }> {
			const resp = await apiRequest<{
				user?: User;
				requires_2fa?: boolean;
				challenge_token?: string;
			}>('/api/auth/login', {
				method: 'POST',
				body: { username: identifier, password }
			});
			if (resp.requires_2fa && resp.challenge_token) {
				return { kind: 'challenge', challenge_token: resp.challenge_token };
			}
			if (!resp.user) throw new Error('unexpected login response');
			set({ user: resp.user, loading: false, initialized: true });
			return { kind: 'session', user: resp.user };
		},

		async completeChallenge(challengeToken: string, code: string): Promise<User> {
			const { user } = await apiRequest<{ user: User }>('/api/auth/login/2fa', {
				method: 'POST',
				body: { challenge_token: challengeToken, code }
			});
			set({ user, loading: false, initialized: true });
			return user;
		},

		async logout() {
			try {
				await apiRequest('/api/auth/logout', { method: 'POST' });
			} catch {
				// Even if logout fails, clear local state.
			}
			set({ user: null, loading: false, initialized: true });
		},

		async setupComplete(input: {
			username: string;
			email: string;
			display_name: string;
			password: string;
		}): Promise<User> {
			const { user } = await apiRequest<{ user: User }>('/api/setup/complete', {
				method: 'POST',
				body: input
			});
			set({ user, loading: false, initialized: true });
			return user;
		}
	};
}

export const auth = createAuthStore();
