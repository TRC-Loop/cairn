// SPDX-License-Identifier: AGPL-3.0-or-later
import { apiRequest } from './client';

export type TwoFAStatus = {
	enabled: boolean;
	enrolled_at: string | null;
	recovery_codes_remaining: number;
};

export type TwoFASetupResponse = {
	secret: string;
	otpauth_url: string;
	qr_code_data_url: string;
};

export type RecoveryCodesResponse = {
	recovery_codes: string[];
};

export function getTwoFAStatus(): Promise<TwoFAStatus> {
	return apiRequest<TwoFAStatus>('/api/auth/2fa/status');
}

export function startTwoFASetup(): Promise<TwoFASetupResponse> {
	return apiRequest<TwoFASetupResponse>('/api/auth/2fa/setup', { method: 'POST' });
}

export function confirmTwoFASetup(code: string): Promise<RecoveryCodesResponse> {
	return apiRequest<RecoveryCodesResponse>('/api/auth/2fa/confirm', {
		method: 'POST',
		body: { code }
	});
}

export function disableTwoFA(currentPassword: string, code: string): Promise<void> {
	return apiRequest('/api/auth/2fa/disable', {
		method: 'POST',
		body: { current_password: currentPassword, code }
	});
}

export function regenerateRecoveryCodes(
	currentPassword: string,
	code: string
): Promise<RecoveryCodesResponse> {
	return apiRequest<RecoveryCodesResponse>('/api/auth/2fa/regenerate-recovery-codes', {
		method: 'POST',
		body: { current_password: currentPassword, code }
	});
}

export function adminReset2FA(userId: number): Promise<void> {
	return apiRequest(`/api/users/${userId}/reset-2fa`, { method: 'POST' });
}

export type LoginResult =
	| { kind: 'session'; user: import('$lib/stores/auth').User }
	| { kind: 'challenge'; challenge_token: string };

export async function loginWithChallenge(
	identifier: string,
	password: string
): Promise<LoginResult> {
	type Response = {
		user?: import('$lib/stores/auth').User;
		requires_2fa?: boolean;
		challenge_token?: string;
	};
	const resp = await apiRequest<Response>('/api/auth/login', {
		method: 'POST',
		body: { username: identifier, password }
	});
	if (resp.requires_2fa && resp.challenge_token) {
		return { kind: 'challenge', challenge_token: resp.challenge_token };
	}
	if (resp.user) {
		return { kind: 'session', user: resp.user };
	}
	throw new Error('unexpected login response');
}

export async function verifyLoginChallenge(
	challengeToken: string,
	code: string
): Promise<import('$lib/stores/auth').User> {
	const { user } = await apiRequest<{ user: import('$lib/stores/auth').User }>(
		'/api/auth/login/2fa',
		{
			method: 'POST',
			body: { challenge_token: challengeToken, code }
		}
	);
	return user;
}
