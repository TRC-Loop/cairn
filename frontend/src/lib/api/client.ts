// SPDX-License-Identifier: AGPL-3.0-or-later

export class ApiError extends Error {
	constructor(
		public status: number,
		public code: string,
		public description: string,
		public fields?: Record<string, string>
	) {
		super(description || code);
	}
}

type RequestOptions = {
	method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';
	body?: unknown;
	headers?: Record<string, string>;
};

export async function apiRequest<T>(path: string, options: RequestOptions = {}): Promise<T> {
	const method = options.method ?? 'GET';
	const headers: Record<string, string> = {
		Accept: 'application/json',
		...options.headers
	};

	if (options.body !== undefined) {
		headers['Content-Type'] = 'application/json';
	}

	if (method !== 'GET') {
		const csrfToken = readCookie('cairn_csrf');
		if (csrfToken) {
			headers['X-CSRF-Token'] = csrfToken;
		}
	}

	const res = await fetch(path, {
		method,
		headers,
		credentials: 'same-origin',
		body: options.body !== undefined ? JSON.stringify(options.body) : undefined
	});

	const contentType = res.headers.get('content-type') ?? '';
	const isJson = contentType.includes('application/json');
	const payload = isJson ? await res.json() : null;

	if (!res.ok) {
		const code = payload?.code ?? 'internal_error';
		const description = payload?.description ?? payload?.error ?? `HTTP ${res.status}`;
		throw new ApiError(res.status, code, description, payload?.fields);
	}

	return payload as T;
}

function readCookie(name: string): string | null {
	if (typeof document === 'undefined') return null;
	const match = document.cookie.match(new RegExp('(^|;\\s*)(' + name + ')=([^;]*)'));
	return match ? decodeURIComponent(match[3]) : null;
}
